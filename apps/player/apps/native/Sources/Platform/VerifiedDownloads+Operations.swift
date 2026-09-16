#if os(iOS)
  import Foundation
  import UIKit

  extension VerifiedDownloads {
    func persist(_ job: VerifiedDownload) throws {
      try store.save(job)
      jobs[job.id] = job
    }

    func snapshot(_ scope: String) async throws -> [VerifiedDownloadSnapshot] {
      try await ready {
        try self.requireScope(scope)
        let preparing = self.plans.values.filter { $0.scope == scope }.map {
          VerifiedDownloadSnapshot(key: $0.key, bytes: 0, total: 0, status: $0.status, message: $0.error, manifest: nil)
        }
        return preparing + self.jobs.values.filter { $0.scope == scope }.map { job in
          let pending = self.tasks.filter { $0.value.id == job.id }.reduce(Int64(0)) { $0 + (self.progress[$1.key] ?? 0) }
          return VerifiedDownloadSnapshot(key: job.key, bytes: min(job.manifest.size, job.bytes + pending), total: job.manifest.size, status: job.status, message: job.error, manifest: job.manifest)
        }
      }
    }

    func pause(_ scope: String, key: String) async throws {
      try await ready {
        try self.requireScope(scope)
        guard OfflineManifest.digest(scope), OfflineManifest.digest(key) else { throw self.failure() }
        if var plan = self.plans[scope + "/" + key] {
          self.cancelPlan(plan.id); plan.status = "paused"; plan.error = ""; try self.savePlan(plan)
        }
        if var job = self.jobs[scope + "/" + key] {
          job.status = "paused"; job.error = ""; try self.persist(job)
          self.probeGenerations[job.id, default: 0] += 1
          self.verifications.removeValue(forKey: job.id)?.cancel(); self.checking.remove(job.id)
          for value in self.tasks.values where value.id == job.id { if value.task.state == .running { value.task.suspend() } }
        }
        self.pump(); self.finishBackgroundEvents()
      }
    }

    func remove(_ scope: String, key: String) async throws {
      try await ready {
        try self.requireScope(scope)
        guard OfflineManifest.digest(scope), OfflineManifest.digest(key) else { throw self.failure() }
        let id = scope + "/" + key
        self.cancelPlan(id); self.cancel(id)
        try self.store.remove(scope: scope, key: key)
        self.plans.removeValue(forKey: id); self.jobs.removeValue(forKey: id)
        self.pump(); self.finishBackgroundEvents()
      }
    }

    func reset() async throws {
      try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
        queue.async { [self] in
          let run = {
            for id in Array(self.jobs.keys) { self.cancel(id) }
            for id in Array(self.plans.keys) { self.cancelPlan(id) }
            do {
              try self.store.reset()
              self.jobs.removeAll(); self.plans.removeAll(); self.checking.removeAll(); self.storageError = false
              continuation.resume()
            } catch { continuation.resume(throwing: error) }
          }
          if restored { run() } else { waiting.append(run) }
        }
      }
    }

    func resume(_ scope: String, key: String, wifiOnly: Bool, quota: Int64) async throws {
      try await ready {
        try self.requireScope(scope)
        _ = try Input.hex(key, count: 64)
        let id = scope + "/" + key
        guard quota == 0 || (1_073_741_824...9_007_199_254_740_991).contains(quota) else { throw ClientError.invalidInput("The download storage limit is invalid.") }
        if let old = self.plans[id] {
          let plan = DownloadPreparation(scope: old.scope, key: old.key, uri: old.uri, kind: old.kind, wifiOnly: wifiOnly, quota: quota, status: "preparing", error: "")
          try self.savePlan(plan)
        } else if var job = self.jobs[id], self.tasks.values.contains(where: { $0.id == id }), job.wifiOnly == wifiOnly {
          let used = self.jobs.values.filter { $0.scope == scope }.reduce(Int64(0)) { $0 + $1.manifest.size }
          guard quota == 0 || used <= quota else { throw VerifiedDownload.failure("The download storage limit has been reached.") }
          job.status = "queued"; job.error = ""; job.retries = 0; job.retryAt = nil
          try self.persist(job)
        } else if let job = self.jobs[id] {
          let input: [String: Any] = try ["scope": job.scope, "key": job.key, "uri": job.uri, "kind": job.kind, "wifiOnly": wifiOnly,
            "quota": quota, "authorization": self.credential(scope, uri: job.uri), "manifest": JSONSerialization.jsonObject(with: JSONEncoder().encode(job.manifest))]
          try self.start(String(decoding: JSONSerialization.data(withJSONObject: input), as: UTF8.self))
        } else { throw ClientError.invalidInput("This download is no longer available.") }
        self.pump()
      }
    }

    func file(_ scope: String, key: String) async throws -> URL {
      try await ready {
        try self.requireScope(scope)
        _ = try Input.hex(key, count: 64)
        guard let job = self.jobs[scope + "/" + key], job.status == "complete" else { throw ClientError.invalidInput("This download has not passed verification.") }
        let url = self.store.media(job)
        guard url.resolvingSymlinksInPath().standardizedFileURL == url.standardizedFileURL,
          try url.resourceValues(forKeys: [.isRegularFileKey, .fileSizeKey]).isRegularFile == true,
          try url.resourceValues(forKeys: [.fileSizeKey]).fileSize == Int(job.manifest.size) else { throw self.failure() }
        guard self.verifiedFiles[job.id] == (try DownloadFileStamp(url)) else { throw ClientError.invalidInput("Open this download again to verify it.") }
        return url
      }
    }

    func check(_ scope: String, key: String) async throws {
      try await ready {
        try self.requireScope(scope)
        guard OfflineManifest.digest(scope), OfflineManifest.digest(key) else { throw self.failure() }
        if let job = self.jobs[scope + "/" + key], job.status == "complete" {
          if self.verifiedFiles[job.id] != (try? DownloadFileStamp(self.store.media(job))) { self.finish(job) }
        }
      }
    }

    func authorize(_ access: DownloadAuthorization, wifiOnly: Bool? = nil, quota: Int64 = 0) async throws {
      try await ready {
        if self.authorization?.scope != access.scope {
          for id in Array(self.jobs.keys) where !id.hasPrefix(access.scope + "/") { self.cancel(id) }
          for id in Array(self.plans.keys) where !id.hasPrefix(access.scope + "/") { self.cancelPlan(id) }
        }
        if let wifiOnly { try self.applyPolicy(scope: access.scope, wifiOnly: wifiOnly, quota: quota) }
        self.authorization = access
        for id in Set(self.tasks.values.filter { $0.id.hasPrefix(access.scope + "/") && $0.task.originalRequest?.value(forHTTPHeaderField: "Authorization") != access.header }.map(\.id)) {
          self.cancel(id)
          if var job = self.jobs[id], !["paused", "complete"].contains(job.status) {
            job.status = "queued"; job.error = ""; job.retries = 0; job.retryAt = nil; try self.persist(job)
          }
        }
        for id in Set(self.planTasks.values.filter { $0.1.hasPrefix(access.scope + "/") && $0.0.originalRequest?.value(forHTTPHeaderField: "Authorization") != access.header }.map { $0.1 }) { self.cancelPlan(id) }
        for (_, transfer) in self.planTasks where transfer.1.hasPrefix(access.scope + "/") { if transfer.0.state == .suspended { transfer.0.resume() } }
        self.pump()
      }
    }

    func updatePolicy(scope: String, wifiOnly: Bool, quota: Int64) async throws {
      try await ready {
        try self.requireScope(scope)
        try self.applyPolicy(scope: scope, wifiOnly: wifiOnly, quota: quota)
        self.pump()
      }
    }

    private func applyPolicy(scope: String, wifiOnly: Bool, quota: Int64) throws {
      _ = try Input.hex(scope, count: 64)
      guard quota == 0 || (1_073_741_824...9_007_199_254_740_991).contains(quota) else { throw ClientError.invalidInput("The download storage limit is invalid.") }
      for var plan in plans.values where plan.scope == scope {
        if plan.wifiOnly != wifiOnly { cancelPlan(plan.id) }
        plan.wifiOnly = wifiOnly; plan.quota = quota; try savePlan(plan)
      }
      let used = jobs.values.filter { $0.scope == scope }.reduce(Int64(0)) { $0 + $1.manifest.size }
      for var job in jobs.values where job.scope == scope {
        let limited = quota > 0 && used > quota && job.status != "complete"
        if job.wifiOnly != wifiOnly || limited { cancel(job.id) }
        job.wifiOnly = wifiOnly
        if limited { job.status = "paused"; job.error = "The download storage limit has been reached." }
        else if job.error == "The download storage limit has been reached." { job.status = "queued"; job.error = "" }
        try persist(job)
      }
    }

    func savePlan(_ plan: DownloadPreparation) throws {
      try store.save(plan)
      plans[plan.id] = plan
    }

    func enqueuePreparation(scope: String, key: String, uri: String, kind: String, wifiOnly: Bool, quota: Int64) async throws {
      try await ready {
        try self.requireScope(scope)
        var plan = DownloadPreparation(scope: scope, key: key, uri: uri, kind: kind, wifiOnly: wifiOnly, quota: quota, status: "preparing", error: "")
        plan.startedAt = Date()
        try plan.validate()
        _ = try self.credential(scope, uri: uri)
        if let old = self.plans[plan.id], old.uri != plan.uri { throw self.failure() }
        guard self.jobs.count + self.plans.count < 1000 || self.plans[plan.id] != nil || self.jobs[plan.id] != nil else { throw self.failure() }
        if self.jobs[plan.id] != nil { return }
        self.cancelPlan(plan.id); try self.savePlan(plan); self.pump()
      }
    }

  }
#endif
