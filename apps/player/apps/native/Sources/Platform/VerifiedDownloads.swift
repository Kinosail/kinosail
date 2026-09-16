#if os(iOS)
  import CryptoKit
  import Foundation
  import UIKit

  /// OS-owned extent transfers; verified blocks live in an app-owned staging file.
  /// The serial utility queue owns journals; a separate bounded worker hashes payloads.
  /// Public async adapters cross that queue; delegate callbacks run on it.
  final class VerifiedDownloads: NSObject, URLSessionDownloadDelegate, @unchecked Sendable {
    static let shared = VerifiedDownloads()
    // State below is confined to queue, including the implementation extensions.
    static let identifier = "com.kinosail.player.offline.swift.v1"
    let queue = DispatchQueue(label: "com.kinosail.player.offline", qos: .utility)
    var jobs: [String: VerifiedDownload] = [:]
    var tasks: [Int: (task: URLSessionDownloadTask, id: String, first: Int, count: Int)] = [:]
    var progress: [Int: Int64] = [:]
    var plans: [String: DownloadPreparation] = [:]
    var planTasks: [Int: (URLSessionDownloadTask, String)] = [:]
    var restored = false
    var waiting: [() -> Void] = []
    var storageError = false
    var playback = false
    var checking = Set<String>()
    let verificationQueue = DispatchQueue(label: "com.kinosail.player.offline.verify", qos: .utility)
    var verifications: [String: DownloadVerification] = [:]
    var verifiedFiles: [String: DownloadFileStamp] = [:]
    var probeGenerations: [String: Int] = [:]
    private var eventsFinished = false
    private var completion: (@MainActor @Sendable () -> Void)?
    var authorization: DownloadAuthorization?
    let store: DownloadJournalStore
    private let configuration: URLSessionConfiguration

    lazy var session: URLSession = {
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
      store = DownloadJournalStore(directory: directory)
      self.configuration = (configuration?.copy() as? URLSessionConfiguration) ?? URLSessionConfiguration.background(withIdentifier: Self.identifier)
      super.init()
      queue.async {
        do {
          let saved = try self.store.load()
          self.jobs = saved.jobs; self.plans = saved.plans
        } catch { self.storageError = true }
        self.session.getAllTasks { found in
          self.queue.async {
            for id in self.jobs.keys {
              if let plan = self.plans.removeValue(forKey: id) {
                try? self.store.removePlan(plan)
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

    func failure() -> ClientError {
      VerifiedDownload.failure("The saved download could not be read. Remove it and try again.")
    }

    func ready<T: Sendable>(_ operation: @escaping @Sendable () throws -> T) async throws -> T {
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

    func credential(_ scope: String, uri: String) throws -> String {
      guard let authorization, authorization.scope == scope else { throw ClientError.http(401) }
      _ = try authorization.server.mediaURL(OfflineResource.url(uri).absoluteString)
      return authorization.header
    }

    func requireScope(_ scope: String) throws {
      guard OfflineManifest.digest(scope), authorization?.scope == scope else { throw ClientError.http(403) }
    }

    func cancel(_ id: String) {
      probeGenerations[id, default: 0] += 1
      verifications.removeValue(forKey: id)?.cancel()
      checking.remove(id); verifiedFiles.removeValue(forKey: id)
      for (number, value) in tasks where value.id == id {
        tasks.removeValue(forKey: number); progress.removeValue(forKey: number); value.task.cancel()
      }
      finishBackgroundEvents()
    }

    func close() async {
      await lock()
      await withCheckedContinuation { continuation in queue.async { [self] in session.invalidateAndCancel(); continuation.resume() } }
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

    func finishBackgroundEvents() {
      guard eventsFinished, checking.isEmpty, let completion else { return }
      eventsFinished = false
      self.completion = nil
      Task { @MainActor in completion() }
    }

    func backgroundCompletion(_ completion: @escaping @MainActor @Sendable () -> Void) {
      queue.async { [self] in self.completion = completion; finishBackgroundEvents() }
    }

  }
#endif
