#if os(iOS)
  import Foundation
  import UIKit

  extension VerifiedDownloadEngine {
    func start(_ raw: String) throws {
      _ = try StrictJSON.decode(Data(raw.utf8))
      guard raw.utf8.count <= 2 * 1024 * 1024, let data = raw.data(using: .utf8),
            let input = try JSONSerialization.jsonObject(with: data) as? [String: Any],
            Set(input.keys) == Set(["scope", "key", "uri", "manifest", "kind", "authorization", "wifiOnly", "quota"]),
            let scope = input["scope"] as? String, let key = input["key"] as? String, let uri = input["uri"] as? String,
            let kind = input["kind"] as? String, let manifestObject = input["manifest"] as? [String: Any],
            Set(manifestObject.keys) == Set(["version", "id", "size", "sha256", "chunkSize", "chunks"]),
            let wifi = input["wifiOnly"] as? NSNumber, CFGetTypeID(wifi) == CFBooleanGetTypeID(),
            let quota = input["quota"] as? NSNumber, CFGetTypeID(quota) != CFBooleanGetTypeID(),
            quota.doubleValue.isFinite, quota.doubleValue.rounded() == quota.doubleValue,
            quota.doubleValue == 0 || (quota.doubleValue >= 1_073_741_824 && quota.doubleValue <= 9_007_199_254_740_991),
            let authorization = input["authorization"] as? String,
            authorization.range(of: "^Bearer [^\\x00-\\x20\\x7f]{1,2048}$", options: .regularExpression) != nil
      else { throw failure() }
      let manifest = try decodeOffline(OfflineManifest.self, from: JSONSerialization.data(withJSONObject: manifestObject))
      try manifest.validate()
      guard try credential(scope, uri: uri) == authorization else { throw ClientError.http(403) }
      var job = VerifiedDownload(scope: scope, key: key, uri: uri, manifest: manifest, kind: kind, wifiOnly: wifi.boolValue,
                                 verified: Array(repeating: false, count: manifest.chunks.count))
      try job.validate()
      if let old = jobs[job.id] {
        guard old.uri == uri, old.manifest == manifest, old.kind == kind else {
          throw VerifiedDownload.failure("This title changed. Remove the download before choosing a new version.")
        }
        job.verified = old.verified
      } else if jobs.count + plans.count >= 1000 && plans[job.id] == nil {
        throw VerifiedDownload.failure("Remove a download before adding another.")
      }
      let used = jobs.values.filter { $0.scope == scope && $0.id != job.id }.reduce(Int64(0)) { $0 + $1.manifest.size }
      guard quota.int64Value == 0 || used + job.manifest.size <= quota.int64Value else {
        throw VerifiedDownload.failure("The download storage limit has been reached.")
      }
      let final = store.media(job), partial = store.staging(job)
      guard final.resolvingSymlinksInPath().standardizedFileURL == final.standardizedFileURL,
            partial.resolvingSymlinksInPath().standardizedFileURL == partial.standardizedFileURL else { throw failure() }
      // Missing/truncated payloads must clear their missing bits before scheduling.
      // Intact blocks still receive a full hash pass before publication.
      let readable = FileManager.default.fileExists(atPath: partial.path) ? partial : final
      let size = (try? readable.resourceValues(forKeys: [.fileSizeKey]).fileSize).map(Int64.init) ?? 0
      for index in job.verified.indices where Int64(index) * manifest.chunkSize + manifest.length(index) > size { job.verified[index] = false }
      let available = try store.availableBytes()
      let reserved = jobs.values.filter { $0.id != job.id && $0.status != "complete" }.reduce(Int64(0)) { $0 + $1.manifest.size - $1.bytes }
      guard let available, available - reserved - 512 * 1024 * 1024 >= job.manifest.size - job.bytes else {
        throw VerifiedDownload.failure("Not enough space. Remove a download and resume.")
      }
      cancel(job.id)
      try store.prepare(job)
      if FileManager.default.fileExists(atPath: final.path), !FileManager.default.fileExists(atPath: partial.path) {
        try FileManager.default.moveItem(at: final, to: partial)
      }
      if !FileManager.default.fileExists(atPath: partial.path) {
        guard FileManager.default.createFile(atPath: partial.path, contents: nil,
                                             attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication]) else { throw failure() }
      }
      let output = try FileHandle(forWritingTo: partial)
      if size > manifest.size { try output.truncate(atOffset: UInt64(manifest.size)) }
      try output.close()
      try persist(job)
      pump()
    }

    func decodeTask(_ value: String) -> (id: String, first: Int, count: Int)? {
      let fields = value.split(separator: ":")
      guard fields.count == 3, let first = Int(fields[1]), let count = Int(fields[2]) else { return nil }
      return (String(fields[0]), first, count)
    }

    private var occupiedSlots: Int { tasks.values.filter { $0.task.state != .suspended }.count + planTasks.count }

    func pump() {
      guard restored, !storageError, !playback, authorization != nil else { return }
      for value in tasks.values.sorted(by: { $0.id < $1.id }) where jobs[value.id]?.status != "paused" && jobs[value.id]?.scope == authorization?.scope {
        if occupiedSlots >= 2 { break }
        if value.task.state == .suspended { value.task.resume() }
      }
      for (_, var job) in jobs.sorted(by: { $0.key < $1.key }) where job.scope == authorization?.scope && ["queued", "waiting", "downloading", "verifying"].contains(job.status) {
        if job.verified.allSatisfy({ $0 }) {
          if !checking.contains(job.id) {
            finish(job)
          }
          continue
        }
        if occupiedSlots >= 2 || tasks.count + planTasks.count >= 100 {
          return
        }
        do {
          let authorization = try credential(job.scope, uri: job.uri)
          let occupied = tasks.values.filter { $0.id == job.id }
          guard let first = job.verified.indices.first(where: { index in
            !job.verified[index] && !occupied.contains(where: { index >= $0.first && index < $0.first + $0.count })
          }) else { continue }
          var count = 1
          while count < (job.retries == 0 ? 8 : 1), first + count < job.verified.count, !job.verified[first + count],
                !occupied.contains(where: { first + count >= $0.first && first + count < $0.first + $0.count })
          {
            count += 1
          }
          let start = Int64(first) * job.manifest.chunkSize
          let end = min(job.manifest.size, Int64(first + count) * job.manifest.chunkSize) - 1
          var request = URLRequest(url: URL(string: job.uri)!, cachePolicy: .reloadIgnoringLocalCacheData)
          request.allowsCellularAccess = !job.wifiOnly
          request.allowsExpensiveNetworkAccess = !job.wifiOnly
          request.setValue(authorization, forHTTPHeaderField: "Authorization")
          request.setValue(self.authorization?.profileID, forHTTPHeaderField: "X-Kinosail-Viewer-Profile")
          request.setValue("identity", forHTTPHeaderField: "Accept-Encoding")
          request.setValue("\"\(job.manifest.sha256)\"", forHTTPHeaderField: "If-Match")
          request.setValue("bytes=\(start)-\(end)", forHTTPHeaderField: "Range")
          let task = session.downloadTask(with: request)
          task.taskDescription = "\(job.id):\(first):\(count)"
          task.countOfBytesClientExpectsToReceive = end - start + 1
          task.priority = URLSessionTask.lowPriority
          if let retryAt = job.retryAt {
            task.earliestBeginDate = retryAt
          }
          job.status = "downloading"
          try persist(job)
          tasks[task.taskIdentifier] = (task, job.id, first, count)
          task.resume()
        } catch {
          job.status = "paused"; job.error = "Sign in again or free storage, then resume."
          do { try persist(job) } catch { storageError = true }
        }
      }
      pumpPlans()
      // Fill the second slot for a single large item, without unbounded tasks.
      if occupiedSlots == 1, planTasks.isEmpty, let active = tasks.values.first(where: { $0.task.state != .suspended }) {
        if let job = jobs[active.id], ["queued", "waiting", "downloading"].contains(job.status), job.verified.indices.contains(where: { !job.verified[$0] && ($0 < active.first || $0 >= active.first + active.count) }) {
          pump()
        }
      }
    }

    func cancelPlan(_ id: String) {
      for (number, value) in planTasks where value.1 == id {
        planTasks.removeValue(forKey: number); value.0.cancel()
      }
    }

    func pumpPlans() {
      for plan in plans.values.sorted(by: { $0.id < $1.id }) where plan.status == "preparing" && plan.scope == authorization?.scope {
        if occupiedSlots >= 2 || tasks.count + planTasks.count >= 100 {
          return
        }
        if planTasks.values.contains(where: { $0.1 == plan.id }) {
          continue
        }
        do {
          var request = URLRequest(url: URL(string: plan.uri.replacingOccurrences(of: "/file", with: "/manifest"))!)
          try request.setValue(credential(plan.scope, uri: plan.uri), forHTTPHeaderField: "Authorization")
          request.setValue("identity", forHTTPHeaderField: "Accept-Encoding")
          request.setValue(authorization?.profileID, forHTTPHeaderField: "X-Kinosail-Viewer-Profile")
          request.allowsCellularAccess = !plan.wifiOnly; request.allowsExpensiveNetworkAccess = !plan.wifiOnly
          let task = session.downloadTask(with: request); task.taskDescription = "plan:" + plan.id; task.countOfBytesClientExpectsToReceive = 2 * 1024 * 1024
          task.earliestBeginDate = plan.retryAt
          planTasks[task.taskIdentifier] = (task, plan.id); task.resume()
        } catch { var failed = plan; failed.status = "paused"; failed.error = "Sign in again to continue preparation."; try? savePlan(failed) }
      }
    }

    func receivedPlan(_ id: String, task: URLSessionDownloadTask, location: URL) {
      guard var plan = plans[id] else { return }
      do {
        guard let response = task.response as? HTTPURLResponse else { throw failure() }
        guard response.url?.absoluteString == plan.uri.replacingOccurrences(of: "/file", with: "/manifest"),
              let size = try location.resourceValues(forKeys: [.fileSizeKey]).fileSize, size >= 0, size <= 2 * 1024 * 1024 else { throw failure() }
        let body = try Data(contentsOf: location)
        let pending = response.statusCode == 503 && (try? StrictJSON.decode(body).object()["error"]) == .string("download is preparing")
        if plan.startedAt == nil { plan.startedAt = Date() }
        guard Date().timeIntervalSince(plan.startedAt!) < 24 * 60 * 60 else { throw failure() }
        if let delay = DownloadRecovery.delay(error: nil, response: response, attempt: pending ? 0 : plan.retries ?? 0) {
          plan.retries = pending ? 0 : (plan.retries ?? 0) + 1
          plan.retryAt = Date(timeIntervalSinceNow: max(15, delay)); try savePlan(plan); return
        }
        guard response.statusCode == 200, response.url?.absoluteString == plan.uri.replacingOccurrences(of: "/file", with: "/manifest"),
              let size = try location.resourceValues(forKeys: [.fileSizeKey]).fileSize, size > 0, size <= 2 * 1024 * 1024 else { throw failure() }
        let manifest = try OfflineManifest(StrictJSON.decode(Data(contentsOf: location)))
        guard try OfflineResource.url(plan.uri).path == "/api/v1/downloads/\(manifest.id)/file" else { throw failure() }
        let input: [String: Any] = try ["scope": plan.scope, "key": plan.key, "uri": plan.uri, "kind": plan.kind, "wifiOnly": plan.wifiOnly, "quota": plan.quota,
                                        "authorization": credential(plan.scope, uri: plan.uri), "manifest": JSONSerialization.jsonObject(with: JSONEncoder().encode(manifest))]
        try start(String(data: JSONSerialization.data(withJSONObject: input), encoding: .utf8)!)
        try store.removePlan(plan); plans.removeValue(forKey: id)
      } catch { plan.status = "paused"; plan.error = "Preparation could not finish. Check Server access and storage, then resume."; try? savePlan(plan) }
    }
  }
#endif
