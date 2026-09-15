#if os(iOS)
  import CryptoKit
  import Foundation
  import UIKit

  /// OS-owned extent transfers; verified blocks live in an app-owned staging file.
  /// The serial utility queue owns journals; a separate bounded worker hashes payloads.
  /// Public async adapters cross that queue; delegate callbacks run on it.
  final class VerifiedDownloads: NSObject, URLSessionDownloadDelegate, @unchecked Sendable {
    static let shared = VerifiedDownloads()
    static let identifier = "com.kinosail.player.offline.swift.v1"
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
    private let verificationQueue = DispatchQueue(label: "com.kinosail.player.offline.verify", qos: .utility)
    private var verifications: [String: DownloadVerification] = [:]
    private var verifiedFiles: [String: DownloadFileStamp] = [:]
    private var probeGenerations: [String: Int] = [:]
    private var eventsFinished = false
    private var completion: (@MainActor @Sendable () -> Void)?
    private var authorization: DownloadAuthorization?
    private let root: URL
    private let configuration: URLSessionConfiguration

    private lazy var session: URLSession = {
      let configuration = self.configuration
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

    init(directory: URL? = nil, configuration: URLSessionConfiguration? = nil) {
      root = directory?.resolvingSymlinksInPath() ?? FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].resolvingSymlinksInPath().appendingPathComponent("kinosail-swift-offline", isDirectory: true)
      self.configuration = (configuration?.copy() as? URLSessionConfiguration) ?? URLSessionConfiguration.background(withIdentifier: Self.identifier)
      super.init()
      queue.async {
        do {
          var metadataBytes = 0
          let scopes = FileManager.default.fileExists(atPath: self.root.path)
            ? try FileManager.default.contentsOfDirectory(at: self.root, includingPropertiesForKeys: nil) : []
          for scope in scopes where OfflineManifest.digest(scope.lastPathComponent) {
            guard scope.resolvingSymlinksInPath().standardizedFileURL == scope.standardizedFileURL,
                  let files = try? FileManager.default.contentsOfDirectory(at: scope, includingPropertiesForKeys: [.fileSizeKey, .isRegularFileKey]) else { continue }
            var deleting = Set<String>()
            for file in files where file.pathExtension == "deleting" && OfflineManifest.digest(file.deletingPathExtension().lastPathComponent) {
              let key = file.deletingPathExtension().lastPathComponent
              deleting.insert(key)
              try? self.deleteFiles(scope: scope.lastPathComponent, key: key)
            }
            for file in files where ["transfer", "planning"].contains(file.pathExtension) && !deleting.contains(file.deletingPathExtension().lastPathComponent) {
              do {
                let values = try file.resourceValues(forKeys: [.fileSizeKey, .isRegularFileKey])
                let size = values.fileSize ?? 0
                guard values.isRegularFile == true, size > 0, size <= 2 * 1024 * 1024,
                      metadataBytes <= 32 * 1024 * 1024 - size, self.jobs.count + self.plans.count < 1000,
                      file.resolvingSymlinksInPath().standardizedFileURL == file.standardizedFileURL else { throw self.failure() }
                metadataBytes += size
                let data = try Data(contentsOf: file)
                if file.pathExtension == "planning" {
                  let plan = try decodeOffline(DownloadPreparation.self, from: data)
                  try plan.validate()
                  guard file == self.planFile(plan), self.plans[plan.id] == nil else { throw self.failure() }
                  self.plans[plan.id] = plan
                } else {
                  let job = try decodeOffline(VerifiedDownload.self, from: data)
                  try job.validate()
                  guard file == self.journal(job), self.jobs[job.id] == nil else { throw self.failure() }
                  self.jobs[job.id] = job
                }
              } catch {
                // Isolate one bad journal; never follow its contents to a different path.
                let quarantine = file.appendingPathExtension("invalid")
                if !FileManager.default.fileExists(atPath: quarantine.path) { try? FileManager.default.moveItem(at: file, to: quarantine) }
              }
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
              guard self.tasks.count + self.planTasks.count < 100, let description = task.taskDescription,
                    description.utf8.count <= 192, task.originalRequest?.httpMethod == "GET" else { task.cancel(); continue }
              if description.hasPrefix("plan:"), let plan = self.plans[String(description.dropFirst(5))], plan.status == "preparing",
                 task.originalRequest?.url?.absoluteString == plan.uri.replacingOccurrences(of: "/file", with: "/manifest"),
                 task.currentRequest?.url == task.originalRequest?.url {
                if task.state == .running { task.suspend() }
                self.planTasks[task.taskIdentifier] = (task, plan.id); continue
              }
              guard let extent = self.decodeTask(description), let job = self.jobs[extent.id],
                    ["queued", "waiting", "downloading", "verifying", "paused"].contains(job.status),
                    extent.first >= 0, extent.count > 0, extent.count <= 8, extent.first <= job.verified.count - extent.count else { task.cancel(); continue }
              let start = Int64(extent.first) * job.manifest.chunkSize
              let end = min(job.manifest.size, Int64(extent.first + extent.count) * job.manifest.chunkSize) - 1
              guard task.originalRequest?.url?.absoluteString == job.uri, task.currentRequest?.url == task.originalRequest?.url,
                    task.originalRequest?.value(forHTTPHeaderField: "Range") == "bytes=\(start)-\(end)",
                    task.originalRequest?.value(forHTTPHeaderField: "If-Match") == "\"\(job.manifest.sha256)\"" else { task.cancel(); continue }
              if task.state == .running { task.suspend() }
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

    private func failure() -> ClientError {
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

    private func ready<T: Sendable>(_ operation: @escaping @Sendable () throws -> T) async throws -> T {
      try await withCheckedThrowingContinuation { continuation in
        queue.async { [self] in
          let run = {
            do { guard !self.storageError else { throw self.failure() }; continuation.resume(returning: try operation()) }
            catch { continuation.resume(throwing: error) }
          }
          if restored { run() } else { waiting.append(run) }
        }
      }
    }

    private func credential(_ scope: String, uri: String) throws -> String {
      guard let authorization, authorization.scope == scope else { throw ClientError.http(401) }
      _ = try authorization.server.mediaURL(OfflineResource.url(uri).absoluteString)
      return authorization.header
    }

    private func requireScope(_ scope: String) throws {
      guard OfflineManifest.digest(scope), authorization?.scope == scope else { throw ClientError.http(403) }
    }

    private func start(_ raw: String) throws {
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
      let final = media(job), partial = staging(job)
      guard final.resolvingSymlinksInPath().standardizedFileURL == final.standardizedFileURL,
            partial.resolvingSymlinksInPath().standardizedFileURL == partial.standardizedFileURL else { throw failure() }
      // Missing/truncated payloads must clear their missing bits before scheduling.
      // Intact blocks still receive a full hash pass before publication.
      let readable = FileManager.default.fileExists(atPath: partial.path) ? partial : final
      let size = (try? readable.resourceValues(forKeys: [.fileSizeKey]).fileSize).map(Int64.init) ?? 0
      for index in job.verified.indices where Int64(index) * manifest.chunkSize + manifest.length(index) > size { job.verified[index] = false }
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
      let output = try FileHandle(forWritingTo: partial)
      if size > manifest.size { try output.truncate(atOffset: UInt64(manifest.size)) }
      try output.close()
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
      verifications.removeValue(forKey: id)?.cancel()
      checking.remove(id); verifiedFiles.removeValue(forKey: id)
      for (number, value) in tasks where value.id == id {
        tasks.removeValue(forKey: number); progress.removeValue(forKey: number); value.task.cancel()
      }
      finishBackgroundEvents()
    }

    private var occupiedSlots: Int { tasks.values.filter { $0.task.state != .suspended }.count + planTasks.count }

    private func pump() {
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
            job.retries = 5; job.status = "paused"; job.error = "Part of the download is missing or damaged. Resume to download that part again."
          }
        } else {
          job.retries = 0; job.retryAt = nil; job.error = ""
        }
        try persist(job)
      } catch {
        job.status = "paused"; job.error = "Could not verify the download. Resume to try again; the parts already checked are kept."
        do { try persist(job) } catch { storageError = true }
      }
    }

    func urlSession(_: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
      if let (_, id) = planTasks.removeValue(forKey: task.taskIdentifier) {
        if let error, var plan = plans[id] {
          if let delay = DownloadRecovery.delay(error: error as NSError, response: task.response as? HTTPURLResponse, attempt: plan.retries ?? 0) {
            plan.retries = (plan.retries ?? 0) + 1
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
      guard !checking.contains(input.id) else { return }
      var job = input
      job.status = "verifying"
      do { try persist(job) } catch { storageError = true; return }
      let path = FileManager.default.fileExists(atPath: staging(job).path) ? staging(job) : media(job)
      let verification = DownloadVerification()
      checking.insert(job.id); verifications[job.id] = verification
      let candidate = job
      let generation = probeGenerations[job.id, default: 0]
      verificationQueue.async {
        let result = Result { try verification.verify(candidate, at: path) }
        self.queue.async {
          guard self.verifications[input.id] === verification else { return }
          switch result {
          case .failure:
            self.completeVerification(input, generation: generation, path: path, playable: false, stamp: nil)
          case .success(let (checked, stamp)):
            guard checked.status == "verifying" else {
              self.verifications.removeValue(forKey: input.id); self.checking.remove(input.id)
              if self.probeGenerations[input.id, default: 0] == generation, self.jobs[input.id]?.status == "verifying" {
                do { try self.persist(checked) } catch { self.storageError = true }
              }
              self.pump(); self.finishBackgroundEvents(); return
            }
            OfflineProbe.check(path, video: checked.kind == "video") { playable in
              self.queue.async {
                guard self.verifications[input.id] === verification else { return }
                self.completeVerification(checked, generation: generation, path: path, playable: playable, stamp: stamp)
              }
            }
          }
        }
      }
    }

    private func completeVerification(_ input: VerifiedDownload, generation: Int, path: URL, playable: Bool, stamp: DownloadFileStamp?) {
      checking.remove(input.id); verifications.removeValue(forKey: input.id)
      defer { pump(); finishBackgroundEvents() }
      guard probeGenerations[input.id, default: 0] == generation, authorization?.scope == input.scope,
            var current = jobs[input.id], current.status == "verifying" else { return }
      do {
        guard let stamp, playable, stamp == (try DownloadFileStamp(path)) else { throw failure() }
        if path != media(current) { try FileManager.default.moveItem(at: path, to: media(current)) }
        current.status = "complete"; current.error = ""
        let finalStamp = try DownloadFileStamp(media(current))
        try persist(current)
        verifiedFiles[current.id] = finalStamp
      } catch {
        current.status = "paused"
        current.error = stamp != nil ? "The file downloaded correctly, but this device could not play it. Download a compatible version." : "The download check stopped. Resume to check again."
        do { try persist(current) } catch { storageError = true }
      }
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
        try self.deleteFiles(scope: scope, key: key)
        self.plans.removeValue(forKey: id); self.jobs.removeValue(forKey: id)
        self.pump(); self.finishBackgroundEvents()
      }
    }

    private func deleteFiles(scope: String, key: String) throws {
      let folder = root.appendingPathComponent(scope)
      try prepare(folder)
      let marker = folder.appendingPathComponent(key + ".deleting")
      guard marker.resolvingSymlinksInPath().standardizedFileURL == marker.standardizedFileURL else { throw failure() }
      try Data().write(to: marker, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
      // Keep the durable intent until both payloads and journals are gone.
      for suffix in [".media", ".media.part", ".transfer", ".planning", ".transfer.invalid", ".planning.invalid"] {
        let file = folder.appendingPathComponent(key + suffix)
        if FileManager.default.fileExists(atPath: file.path) { try FileManager.default.removeItem(at: file) }
      }
      try FileManager.default.removeItem(at: marker)
    }

    func close() async {
      await lock()
      await withCheckedContinuation { continuation in queue.async { [self] in session.invalidateAndCancel(); continuation.resume() } }
    }

    func reset() async throws {
      try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
        queue.async { [self] in
          let run = {
            for id in Array(self.jobs.keys) { self.cancel(id) }
            for id in Array(self.plans.keys) { self.cancelPlan(id) }
            do {
              guard self.root.resolvingSymlinksInPath().standardizedFileURL == self.root.standardizedFileURL else { throw self.failure() }
              if FileManager.default.fileExists(atPath: self.root.path) { try FileManager.default.removeItem(at: self.root) }
              self.jobs.removeAll(); self.plans.removeAll(); self.checking.removeAll(); self.storageError = false
              continuation.resume()
            } catch { continuation.resume(throwing: error) }
          }
          if restored { run() } else { waiting.append(run) }
        }
      }
    }

    func lock() async {
      await withCheckedContinuation { continuation in
        queue.async { [self] in
          authorization = nil
          for id in Array(jobs.keys) { cancel(id) }
          for id in Array(plans.keys) { cancelPlan(id) }
          continuation.resume()
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
        let url = self.media(job)
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
          if self.verifiedFiles[job.id] != (try? DownloadFileStamp(self.media(job))) { self.finish(job) }
        }
      }
    }

    func setPlayback(_ active: Bool) {
      queue.async {
        self.playback = active
        if active {
          for id in Set(self.tasks.values.map(\.id)) {
            for value in self.tasks.values where value.id == id { if value.task.state == .running { value.task.suspend() } }
            if var job = self.jobs[id], job.status != "paused" {
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
      guard eventsFinished, checking.isEmpty, let completion else { return }
      eventsFinished = false
      self.completion = nil
      Task { @MainActor in completion() }
    }

    func backgroundCompletion(_ completion: @escaping @MainActor @Sendable () -> Void) {
      queue.async { [self] in self.completion = completion; finishBackgroundEvents() }
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

    func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
                    newRequest request: URLRequest, completionHandler: @escaping @Sendable (URLRequest?) -> Void) { completionHandler(nil) }

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

    private func pumpPlans() {
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

    private func receivedPlan(_ id: String, task: URLSessionDownloadTask, location: URL) {
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
        try FileManager.default.removeItem(at: planFile(plan)); plans.removeValue(forKey: id)
      } catch { plan.status = "paused"; plan.error = "Preparation could not finish. Check Server access and storage, then resume."; try? savePlan(plan) }
    }
  }
#endif
