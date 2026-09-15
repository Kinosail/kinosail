#if os(iOS)
  import CryptoKit
  import ExpoModulesCore
  import Foundation
  import Security
  import UIKit

  /// OS-owned extent transfers; verified blocks live in an app-owned staging file.
  /// The serial utility queue owns every journal mutation and all disk/hash work.
  final class VerifiedDownloads: NSObject, URLSessionDownloadDelegate {
    static let shared = VerifiedDownloads()
    static let identifier = "com.kinosail.player.offline.v2"
    private let queue = DispatchQueue(label: "com.kinosail.player.offline", qos: .utility)
    private var jobs: [String: VerifiedDownload] = [:]
    private var tasks: [Int: (task: URLSessionDownloadTask, id: String, first: Int, count: Int)] = [:]
    private var progress: [Int: Int64] = [:]
    private var plans: [String: DownloadPreparation] = [:]
    private var planTasks: [Int: (URLSessionDownloadTask, String)] = [:]
    private var restored = false
    private var waiting: [() -> Void] = []
    private var storageError = false
    private var playback = false
    private var checking = Set<String>()
    private var probeGenerations: [String: Int] = [:]
    private var eventsFinished = false
    var completion: (() -> Void)?
    private var root: URL {
      FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("kinosail-offline", isDirectory: true)
    }

    private lazy var session: URLSession = {
      let configuration = URLSessionConfiguration.background(withIdentifier: Self.identifier)
      configuration.sessionSendsLaunchEvents = true
      configuration.isDiscretionary = false
      configuration.waitsForConnectivity = true
      configuration.httpMaximumConnectionsPerHost = 2
      configuration.timeoutIntervalForRequest = 60
      configuration.timeoutIntervalForResource = 7 * 24 * 60 * 60
      configuration.httpCookieStorage = nil
      configuration.urlCredentialStorage = nil
      configuration.urlCache = nil
      configuration.httpShouldSetCookies = false
      let delegates = OperationQueue()
      delegates.maxConcurrentOperationCount = 1
      delegates.underlyingQueue = queue
      return URLSession(configuration: configuration, delegate: self, delegateQueue: delegates)
    }()

    override private init() {
      super.init()
      queue.async {
        do {
          var metadataBytes = 0
          for scope in (try? FileManager.default.contentsOfDirectory(at: self.root, includingPropertiesForKeys: nil)) ?? [] where DownloadManifest.digest(scope.lastPathComponent) {
            for file in try FileManager.default.contentsOfDirectory(at: scope, includingPropertiesForKeys: [.fileSizeKey]) where file.lastPathComponent.hasSuffix(".transfer") || file.lastPathComponent.hasSuffix(".planning") {
              let size = try file.resourceValues(forKeys: [.fileSizeKey]).fileSize ?? 0
              metadataBytes += size
              guard size > 0, size <= 2 * 1024 * 1024, metadataBytes <= 32 * 1024 * 1024, self.jobs.count + self.plans.count < 1000,
                    file.resolvingSymlinksInPath().standardizedFileURL == file.standardizedFileURL else { throw self.failure() }
              let data = try Data(contentsOf: file)
              if file.lastPathComponent.hasSuffix(".planning") {
                let plan = try decodeOffline(DownloadPreparation.self, from: data)
                try plan.validate()
                guard file == self.planFile(plan), self.plans[plan.id] == nil else { throw self.failure() }
                self.plans[plan.id] = plan; continue
              }
              let job = try decodeOffline(VerifiedDownload.self, from: data)
              try job.validate()
              guard file == self.journal(job), self.jobs[job.id] == nil else { throw self.failure() }
              self.jobs[job.id] = job
            }
          }
        } catch { self.storageError = true }
        self.session.getAllTasks { found in
          self.queue.async {
            for id in self.jobs.keys {
              if let plan = self.plans.removeValue(forKey: id) {
                try? FileManager.default.removeItem(at: self.planFile(plan))
              }
            }
            for case let task as URLSessionDownloadTask in found {
              if let description = task.taskDescription, description.hasPrefix("plan:"), let plan = self.plans[String(description.dropFirst(5))], plan.status == "preparing" {
                self.planTasks[task.taskIdentifier] = (task, plan.id); continue
              }
              guard let description = task.taskDescription, let extent = self.decodeTask(description),
                    let job = self.jobs[extent.id], !["paused", "complete"].contains(job.status),
                    extent.first >= 0, extent.count > 0, extent.count <= 8, extent.first <= job.verified.count - extent.count
              else { task.cancel(); continue }
              self.tasks[task.taskIdentifier] = (task, extent.id, extent.first, extent.count)
            }
            for (id, var job) in self.jobs where job.status != "complete" {
              if !self.tasks.values.contains(where: { $0.id == id }), job.status != "paused" {
                job.status = "queued"
              }
              self.jobs[id] = job
            }
            self.restored = true
            let pending = self.waiting; self.waiting.removeAll()
            pending.forEach { $0() }
            self.pump()
          }
        }
      }
    }

    private func failure() -> NSError {
      VerifiedDownload.failure("The saved download could not be read. Remove it and try again.")
    }

    private func directory(_ job: VerifiedDownload) -> URL {
      root.appendingPathComponent(job.scope, isDirectory: true)
    }

    private func journal(_ job: VerifiedDownload) -> URL {
      directory(job).appendingPathComponent(job.key + ".transfer")
    }

    private func media(_ job: VerifiedDownload) -> URL {
      directory(job).appendingPathComponent(job.key + ".media")
    }

    private func staging(_ job: VerifiedDownload) -> URL {
      directory(job).appendingPathComponent(job.key + ".media.part")
    }

    private func prepare(_ url: URL) throws {
      guard url.resolvingSymlinksInPath().standardizedFileURL == url.standardizedFileURL else { throw failure() }
      try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true,
                                              attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication])
      var url = url
      var values = URLResourceValues(); values.isExcludedFromBackup = true
      try url.setResourceValues(values)
    }

    private func persist(_ job: VerifiedDownload) throws {
      try job.validate()
      try prepare(directory(job))
      try JSONEncoder().encode(job).write(to: journal(job), options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
      jobs[job.id] = job
    }

    private func ready(_ promise: Promise, _ operation: @escaping () throws -> Any?) {
      queue.async {
        let run = {
          do { guard !self.storageError else { throw self.failure() }; try promise.resolve(operation()) }
          catch { promise.reject("DOWNLOAD_FAILED", error.localizedDescription) }
        }
        if self.restored {
          run()
        } else {
          self.waiting.append(run)
        }
      }
    }

    private func credential(_ scope: String, _ value: String? = nil) throws -> String {
      let query: [String: Any] = [kSecClass as String: kSecClassGenericPassword,
                                  kSecAttrService as String: Self.identifier, kSecAttrAccount as String: scope]
      if let value {
        let data = Data(value.utf8)
        let status = SecItemUpdate(query as CFDictionary, [kSecValueData as String: data] as CFDictionary)
        if status == errSecItemNotFound {
          var insert = query
          insert[kSecValueData as String] = data
          insert[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
          guard SecItemAdd(insert as CFDictionary, nil) == errSecSuccess else { throw failure() }
        } else if status != errSecSuccess {
          throw failure()
        }
        return value
      }
      var lookup = query
      lookup[kSecReturnData as String] = true
      var result: CFTypeRef?
      guard SecItemCopyMatching(lookup as CFDictionary, &result) == errSecSuccess,
            let data = result as? Data, let value = String(data: data, encoding: .utf8)
      else {
        throw VerifiedDownload.failure("Sign in again to resume this download.")
      }
      return value
    }

    func enqueue(_ raw: String, promise: Promise) {
      ready(promise) { try self.start(raw); return nil }
    }

    private func start(_ raw: String) throws {
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
      let manifest = try decodeOffline(DownloadManifest.self, from: JSONSerialization.data(withJSONObject: manifestObject))
      try manifest.validate()
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
      let used = jobs.values.filter { $0.id != job.id }.reduce(Int64(0)) { $0 + $1.manifest.size }
      guard quota.int64Value == 0 || used + job.manifest.size <= quota.int64Value else {
        throw VerifiedDownload.failure("The download storage limit has been reached.")
      }
      let final = media(job), partial = staging(job)
      guard final.resolvingSymlinksInPath().standardizedFileURL == final.standardizedFileURL,
            partial.resolvingSymlinksInPath().standardizedFileURL == partial.standardizedFileURL else { throw failure() }
      let readable = FileManager.default.fileExists(atPath: partial.path) ? partial : final
      if FileManager.default.fileExists(atPath: readable.path) {
        // Revalidate saved blocks (and adopt matching legacy files) before resume.
        let file = try FileHandle(forReadingFrom: readable); defer { try? file.close() }
        for index in job.verified.indices {
          try file.seek(toOffset: UInt64(Int64(index) * manifest.chunkSize))
          let bytes = try file.read(upToCount: Int(manifest.length(index))) ?? Data()
          job.verified[index] = bytes.count == Int(manifest.length(index)) && VerifiedDownload.hash(bytes) == manifest.chunks[index]
        }
      }
      let available = try FileManager.default.attributesOfFileSystem(forPath: root.deletingLastPathComponent().path)[.systemFreeSize] as? NSNumber
      let reserved = jobs.values.filter { $0.id != job.id && $0.status != "complete" }.reduce(Int64(0)) { $0 + $1.manifest.size - $1.bytes }
      guard let available, available.int64Value - reserved - 512 * 1024 * 1024 >= job.manifest.size - job.bytes else {
        throw VerifiedDownload.failure("Not enough space. Remove a download and resume.")
      }
      cancel(job.id)
      try prepare(directory(job))
      if FileManager.default.fileExists(atPath: final.path), !FileManager.default.fileExists(atPath: partial.path) {
        try FileManager.default.moveItem(at: final, to: partial)
      }
      if !FileManager.default.fileExists(atPath: partial.path) {
        guard FileManager.default.createFile(atPath: partial.path, contents: nil,
                                             attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication]) else { throw failure() }
      }
      _ = try credential(scope, authorization)
      try persist(job)
      pump()
    }

    private func decodeTask(_ value: String) -> (id: String, first: Int, count: Int)? {
      let fields = value.split(separator: ":")
      guard fields.count == 3, let first = Int(fields[1]), let count = Int(fields[2]) else { return nil }
      return (String(fields[0]), first, count)
    }

    private func cancel(_ id: String) {
      probeGenerations[id, default: 0] += 1
      for (number, value) in tasks where value.id == id {
        tasks.removeValue(forKey: number); progress.removeValue(forKey: number); value.task.cancel()
      }
    }

    private func pump() {
      guard restored, !storageError, !playback else { return }
      for (_, var job) in jobs.sorted(by: { $0.key < $1.key }) where ["queued", "waiting", "downloading", "verifying"].contains(job.status) {
        if job.verified.allSatisfy({ $0 }) {
          if !checking.contains(job.id) {
            finish(job)
          }
          continue
        }
        if tasks.count + planTasks.count >= 2 {
          return
        }
        do {
          let authorization = try credential(job.scope)
          let occupied = tasks.values.filter { $0.id == job.id }
          guard let first = job.verified.indices.first(where: { index in
            !job.verified[index] && !occupied.contains(where: { index >= $0.first && index < $0.first + $0.count })
          }) else { continue }
          var count = 1
          while count < 8, first + count < job.verified.count, !job.verified[first + count],
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
      if tasks.count == 1 {
        let active = tasks.values.first!
        if let job = jobs[active.id], ["queued", "waiting", "downloading"].contains(job.status), job.verified.indices.contains(where: { !job.verified[$0] && ($0 < active.first || $0 >= active.first + active.count) }) {
          pump()
        }
      }
    }

    func urlSession(_: URLSession, downloadTask: URLSessionDownloadTask, didWriteData _: Int64,
                    totalBytesWritten: Int64, totalBytesExpectedToWrite: Int64)
    {
      if planTasks[downloadTask.taskIdentifier] != nil {
        if totalBytesWritten > 2 * 1024 * 1024 || totalBytesExpectedToWrite > 2 * 1024 * 1024 {
          downloadTask.cancel()
        }; return
      }
      guard let extent = tasks[downloadTask.taskIdentifier], let job = jobs[extent.id] else { return }
      let length = min(job.manifest.size, Int64(extent.first + extent.count) * job.manifest.chunkSize) - Int64(extent.first) * job.manifest.chunkSize
      if totalBytesWritten > length || totalBytesExpectedToWrite > length {
        downloadTask.cancel(); return
      }
      progress[downloadTask.taskIdentifier] = max(0, totalBytesWritten)
    }

    func urlSession(_: URLSession, downloadTask: URLSessionDownloadTask, didFinishDownloadingTo location: URL) {
      if let (_, id) = planTasks[downloadTask.taskIdentifier] {
        receivedPlan(id, task: downloadTask, location: location); return
      }
      guard let extent = tasks[downloadTask.taskIdentifier], var job = jobs[extent.id] else { return }
      do {
        let start = Int64(extent.first) * job.manifest.chunkSize
        let end = min(job.manifest.size, Int64(extent.first + extent.count) * job.manifest.chunkSize) - 1
        guard let response = downloadTask.response as? HTTPURLResponse else { throw failure() }
        if let delay = DownloadRecovery.delay(error: nil, response: response, attempt: job.retries) {
          job.retries += 1; job.retryAt = Date(timeIntervalSinceNow: delay); job.status = "waiting"; job.error = "The Server is busy. Retrying automatically."
          try persist(job); return
        }
        guard response.statusCode == 206, response.url?.absoluteString == job.uri,
              response.value(forHTTPHeaderField: "ETag") == "\"\(job.manifest.sha256)\"",
              response.value(forHTTPHeaderField: "Content-Range") == "bytes \(start)-\(end)/\(job.manifest.size)",
              response.value(forHTTPHeaderField: "Content-Length") == String(end - start + 1),
              try location.resourceValues(forKeys: [.fileSizeKey]).fileSize == Int(end - start + 1)
        else { throw failure() }
        let input = try FileHandle(forReadingFrom: location); defer { try? input.close() }
        let output = try FileHandle(forWritingTo: staging(job)); defer { try? output.close() }
        var corrupt = false
        for index in extent.first ..< (extent.first + extent.count) {
          let block = try input.read(upToCount: Int(job.manifest.length(index))) ?? Data()
          guard block.count == Int(job.manifest.length(index)), VerifiedDownload.hash(block) == job.manifest.chunks[index] else { corrupt = true; continue }
          try output.seek(toOffset: UInt64(Int64(index) * job.manifest.chunkSize))
          try output.write(contentsOf: block)
          try output.synchronize()
          job.verified[index] = true
          try persist(job) // Data is durable before its verified bit.
        }
        if corrupt {
          job.retries += 1; if job.retries > 5 {
            job.retries = 5; job.status = "paused"; job.error = "Integrity check failed. Resume to repair the missing blocks."
          }
        } else {
          job.retries = 0; job.retryAt = nil; job.error = ""
        }
        try persist(job)
      } catch {
        job.status = "paused"; job.error = "Transfer could not be verified. Resume to retry; verified data is kept."
        do { try persist(job) } catch { storageError = true }
      }
    }

    func urlSession(_: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
      if let (_, id) = planTasks.removeValue(forKey: task.taskIdentifier) {
        if let error, var plan = plans[id] {
          if let delay = DownloadRecovery.delay(error: error as NSError, response: task.response as? HTTPURLResponse, attempt: 0) {
            plan.retryAt = Date(timeIntervalSinceNow: max(15, delay))
          } else {
            plan.status = "paused"; plan.error = "Preparation interrupted. Resume to try again."
          }
          try? savePlan(plan)
        }
        pump(); return
      }
      guard let extent = tasks.removeValue(forKey: task.taskIdentifier), var job = jobs[extent.id] else { return }
      progress.removeValue(forKey: task.taskIdentifier)
      if let error, job.status != "paused" {
        if let delay = DownloadRecovery.delay(error: error as NSError, response: task.response as? HTTPURLResponse, attempt: job.retries) {
          job.retryAt = Date(timeIntervalSinceNow: delay); job.retries += 1; job.status = "waiting"; job.error = "Connection interrupted. Retrying automatically."
        } else {
          job.status = "paused"; job.error = "Download interrupted. Resume to continue."
        }
        do { try persist(job) } catch { storageError = true }
      }
      pump()
    }

    private func finish(_ input: VerifiedDownload) {
      var job = input
      checking.insert(job.id)
      job.status = "verifying"
      do {
        try persist(job)
        let path = FileManager.default.fileExists(atPath: staging(job).path) ? staging(job) : media(job)
        guard try path.resourceValues(forKeys: [.fileSizeKey]).fileSize == Int(job.manifest.size) else { throw failure() }
        let file = try FileHandle(forReadingFrom: path); defer { try? file.close() }
        var whole = SHA256()
        for index in job.verified.indices {
          let block = try file.read(upToCount: Int(job.manifest.length(index))) ?? Data()
          job.verified[index] = block.count == Int(job.manifest.length(index)) && VerifiedDownload.hash(block) == job.manifest.chunks[index]
          whole.update(data: block)
        }
        guard job.verified.allSatisfy({ $0 }), whole.finalize().map({ String(format: "%02x", $0) }).joined() == job.manifest.sha256 else {
          job.status = "paused"; job.error = "Integrity check failed. Resume to repair the missing blocks."
          try persist(job); checking.remove(job.id); return
        }
        let probeID = job.id
        let generation = probeGenerations[probeID, default: 0]
        OfflineProbe.check(path, video: job.kind == "video") { playable in
          self.queue.async {
            self.checking.remove(probeID)
            defer { self.pump(); self.finishBackgroundEvents() }
            guard self.probeGenerations[probeID, default: 0] == generation, let current = self.jobs[probeID], current.status == "verifying" else { return }
            var result = current
            do {
              guard playable else { throw VerifiedDownload.failure("This file cannot play locally. Choose a compatible download quality.") }
              if path != self.media(result) {
                try FileManager.default.moveItem(at: path, to: self.media(result))
              }
              result.status = "complete"; result.error = ""
              try self.persist(result)
            } catch {
              result.status = "paused"; result.error = "File verified, but local playback failed. Choose a compatible download quality."
              do { try self.persist(result) } catch { self.storageError = true }
            }
          }
        }
      } catch {
        checking.remove(job.id)
        job.status = "paused"; job.error = "Local verification stopped. Resume to check again."
        do { try persist(job) } catch { storageError = true }
      }
    }

    func snapshot(_ scope: String, promise: Promise) {
      ready(promise) {
        guard DownloadManifest.digest(scope) else { throw self.failure() }
        let preparing: [[String: Any]] = self.plans.values.filter { $0.scope == scope }.map { ["key": $0.key, "bytes": 0, "total": 0, "status": $0.status, "error": $0.error] }
        return preparing + self.jobs.values.filter { $0.scope == scope }.map { job -> [String: Any] in
          let pending = self.tasks.filter { $0.value.id == job.id }.reduce(Int64(0)) { $0 + (self.progress[$1.key] ?? 0) }
          return ["key": job.key, "bytes": min(job.manifest.size, job.bytes + pending), "total": job.manifest.size, "status": job.status, "error": job.error, "manifest": (try? JSONSerialization.jsonObject(with: JSONEncoder().encode(job.manifest))) ?? [:]]
        }
      }
    }

    func pause(_ scope: String, key: String, promise: Promise) {
      ready(promise) {
        guard DownloadManifest.digest(scope), DownloadManifest.digest(key) else { throw self.failure() }
        if var plan = self.plans[scope + "/" + key] {
          self.cancelPlan(plan.id); plan.status = "paused"; plan.error = ""; try self.savePlan(plan)
        }
        if var job = self.jobs[scope + "/" + key] {
          self.cancel(job.id); job.status = "paused"; job.error = ""; try self.persist(job)
        }
        self.pump(); return nil
      }
    }

    func remove(_ scope: String, key: String, promise: Promise) {
      ready(promise) {
        guard DownloadManifest.digest(scope), DownloadManifest.digest(key) else { throw self.failure() }
        if let plan = self.plans.removeValue(forKey: scope + "/" + key) {
          self.cancelPlan(plan.id); try FileManager.default.removeItem(at: self.planFile(plan))
        }
        if let job = self.jobs[scope + "/" + key] {
          self.cancel(job.id)
          for file in [self.journal(job), self.media(job), self.staging(job)] where FileManager.default.fileExists(atPath: file.path) {
            try FileManager.default.removeItem(at: file)
          }
          self.jobs.removeValue(forKey: job.id)
        }
        self.pump(); return nil
      }
    }

    func clear(_ promise: Promise) {
      queue.async {
        let run = {
          for id in self.jobs.keys {
            self.probeGenerations[id, default: 0] += 1
          }
          self.tasks.values.forEach { $0.task.cancel() }; self.tasks.removeAll(); self.progress.removeAll()
          self.planTasks.values.forEach { $0.0.cancel() }; self.planTasks.removeAll(); self.plans.removeAll()
          self.jobs.removeAll(); self.checking.removeAll()
          SecItemDelete([kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: Self.identifier] as CFDictionary)
          do {
            if FileManager.default.fileExists(atPath: self.root.path) {
              try FileManager.default.removeItem(at: self.root)
            }
            self.storageError = false; promise.resolve(nil)
          } catch { promise.reject("DOWNLOAD_FAILED", "Downloads could not be removed.") }
        }
        if self.restored {
          run()
        } else {
          self.waiting.append(run)
        }
      }
    }

    func check(_ scope: String, key: String, promise: Promise) {
      ready(promise) {
        guard DownloadManifest.digest(scope), DownloadManifest.digest(key) else { throw self.failure() }
        if let job = self.jobs[scope + "/" + key], job.status == "complete" {
          self.finish(job)
        }
        return nil
      }
    }

    func setPlayback(_ active: Bool) {
      queue.async {
        self.playback = active
        if active {
          for id in Set(self.tasks.values.map(\.id)) {
            self.cancel(id)
            if var job = self.jobs[id] {
              job.status = "waiting"; job.error = "Downloads resume after playback."; try? self.persist(job)
            }
          }
        } else {
          self.pump()
        }
      }
    }

    func urlSessionDidFinishEvents(forBackgroundURLSession _: URLSession) {
      eventsFinished = true
      finishBackgroundEvents()
    }

    private func finishBackgroundEvents() {
      guard eventsFinished, checking.isEmpty else { return }
      eventsFinished = false
      DispatchQueue.main.async { let done = self.completion; self.completion = nil; done?() }
    }

    func authorize(_ scope: String, authorization: String, promise: Promise) {
      ready(promise) {
        guard DownloadManifest.digest(scope), authorization.range(of: "^Bearer [^\\x00-\\x20\\x7f]{1,2048}$", options: .regularExpression) != nil else { throw self.failure() }
        _ = try self.credential(scope, authorization)
        for (id, var job) in self.jobs where job.scope == scope && ["queued", "waiting", "downloading"].contains(job.status) {
          self.cancel(id); job.status = "queued"; job.error = ""; job.retries = 0; job.retryAt = nil; try self.persist(job)
        }
        self.pump(); return nil
      }
    }

    private func planFile(_ plan: DownloadPreparation) -> URL {
      root.appendingPathComponent(plan.scope).appendingPathComponent(plan.key + ".planning")
    }

    private func savePlan(_ plan: DownloadPreparation) throws {
      try plan.validate(); try prepare(root.appendingPathComponent(plan.scope))
      try JSONEncoder().encode(plan).write(to: planFile(plan), options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
      plans[plan.id] = plan
    }

    private func cancelPlan(_ id: String) {
      for (number, value) in planTasks where value.1 == id {
        planTasks.removeValue(forKey: number); value.0.cancel()
      }
    }

    func enqueuePreparation(_ raw: String, promise: Promise) {
      ready(promise) {
        guard raw.utf8.count <= 8192, let data = raw.data(using: .utf8), let input = try JSONSerialization.jsonObject(with: data) as? [String: Any],
              Set(input.keys) == Set(["scope", "key", "uri", "kind", "wifiOnly", "quota", "authorization"]),
              let authorization = input["authorization"] as? String,
              authorization.range(of: "^Bearer [^\\x00-\\x20\\x7f]{1,2048}$", options: .regularExpression) != nil else { throw self.failure() }
        var fields = input; fields.removeValue(forKey: "authorization")
        fields["status"] = "preparing"; fields["error"] = ""
        let plan = try decodeOffline(DownloadPreparation.self, from: JSONSerialization.data(withJSONObject: fields))
        try plan.validate()
        if let old = self.plans[plan.id], old.uri != plan.uri {
          throw self.failure()
        }
        guard self.plans.count + self.jobs.count + self.plans.count < 1000 || self.plans[plan.id] != nil || self.jobs[plan.id] != nil else { throw self.failure() }
        if self.jobs[plan.id] != nil {
          return nil
        }
        _ = try self.credential(plan.scope, authorization)
        self.cancelPlan(plan.id); try self.savePlan(plan); self.pump(); return nil
      }
    }

    private func pumpPlans() {
      for plan in plans.values.sorted(by: { $0.id < $1.id }) where plan.status == "preparing" {
        if tasks.count + planTasks.count >= 2 {
          return
        }
        if planTasks.values.contains(where: { $0.1 == plan.id }) {
          continue
        }
        do {
          var request = URLRequest(url: URL(string: plan.uri.replacingOccurrences(of: "/file", with: "/manifest"))!)
          try request.setValue(credential(plan.scope), forHTTPHeaderField: "Authorization")
          request.setValue("identity", forHTTPHeaderField: "Accept-Encoding")
          request.allowsCellularAccess = !plan.wifiOnly; request.allowsExpensiveNetworkAccess = !plan.wifiOnly
          let task = session.downloadTask(with: request); task.taskDescription = "plan:" + plan.id; task.countOfBytesClientExpectsToReceive = 2 * 1024 * 1024
          task.earliestBeginDate = plan.retryAt
          planTasks[task.taskIdentifier] = (task, plan.id); task.resume()
        } catch { var failed = plan; failed.status = "paused"; failed.error = "Sign in again to continue preparation."; try? savePlan(failed) }
      }
    }

    private func receivedPlan(_ id: String, task: URLSessionDownloadTask, location: URL) {
      guard var plan = plans[id] else { return }
      do {
        guard let response = task.response as? HTTPURLResponse else { throw failure() }
        if let delay = DownloadRecovery.delay(error: nil, response: response, attempt: 0) {
          plan.retryAt = Date(timeIntervalSinceNow: max(15, delay)); try savePlan(plan); return
        }
        guard response.statusCode == 200, response.url?.absoluteString == plan.uri.replacingOccurrences(of: "/file", with: "/manifest"),
              let size = try location.resourceValues(forKeys: [.fileSizeKey]).fileSize, size > 0, size <= 2 * 1024 * 1024 else { throw failure() }
        let manifest = try decodeOffline(DownloadManifest.self, from: Data(contentsOf: location)); try manifest.validate()
        let input: [String: Any] = try ["scope": plan.scope, "key": plan.key, "uri": plan.uri, "kind": plan.kind, "wifiOnly": plan.wifiOnly, "quota": plan.quota,
                                        "authorization": credential(plan.scope), "manifest": JSONSerialization.jsonObject(with: JSONEncoder().encode(manifest))]
        try start(String(data: JSONSerialization.data(withJSONObject: input), encoding: .utf8)!)
        try FileManager.default.removeItem(at: planFile(plan)); plans.removeValue(forKey: id)
      } catch { plan.status = "paused"; plan.error = "Preparation could not finish. Check Server access and storage, then resume."; try? savePlan(plan) }
    }
  }
#endif
