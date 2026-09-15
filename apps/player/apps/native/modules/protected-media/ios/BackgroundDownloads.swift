#if os(iOS)
import ExpoModulesCore
import Foundation
import CryptoKit
import UIKit

// A single OS-owned session survives React reloads and application suspension.
// Media/progress stays in the existing JS index; this journal owns only transfers.
final class BackgroundDownloads: NSObject, URLSessionDownloadDelegate {
  static let shared = BackgroundDownloads()
  static let identifier = "com.kinosail.player.offline.v1"
  private let limit: Int64 = 9_007_199_254_740_991 // Largest exact JavaScript byte count.
  private let reserve: Int64 = 512 * 1024 * 1024
  private var jobs: [String: Job] = [:]
  private var tasks: [String: URLSessionDownloadTask] = [:]
  private var waiting: [() -> Void] = []
  private var restored = false
  private var storageError = false
  var completion: (() -> Void)?
  private struct Job: Codable {
    var scope: String
    var key: String
    var uri: String
    var etag: String?
    var modified: String
    var total: Int64
    var bytes: Int64 = 0
    var status = "queued"
    var error = ""
    var taskID: Int = 0
    var retries: Int?
    var wifiOnly: Bool?
    var id: String { scope + "/" + key }
  }
  private var root: URL {
    FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].resolvingSymlinksInPath()
      .appendingPathComponent("kinosail-offline", isDirectory: true)
  }
  private lazy var session: URLSession = {
    let config = URLSessionConfiguration.background(withIdentifier: Self.identifier)
    config.sessionSendsLaunchEvents = true
    config.isDiscretionary = false
    config.waitsForConnectivity = true
    config.httpMaximumConnectionsPerHost = 2
    config.timeoutIntervalForRequest = 60
    config.timeoutIntervalForResource = 7 * 24 * 60 * 60
    config.urlCredentialStorage = nil
    config.urlCache = nil
    config.httpCookieStorage = nil
    config.httpShouldSetCookies = false
    return URLSession(configuration: config, delegate: self, delegateQueue: .main)
  }()
  private override init() {
    super.init()
    do {
      guard root.resolvingSymlinksInPath().standardizedFileURL == root.standardizedFileURL else { throw failure() }
      let journal = root.appendingPathComponent("transfers.json")
      if FileManager.default.fileExists(atPath: journal.path) {
        let size = try journal.resourceValues(forKeys: [.fileSizeKey]).fileSize ?? 0
        guard size <= 1024 * 1024 else { throw failure() }
        let data = try Data(contentsOf: journal)
        guard let values = try JSONSerialization.jsonObject(with: data) as? [[String: Any]], values.count <= 50,
          values.allSatisfy({ Set($0.keys).subtracting(["wifiOnly", "retries", "etag"]) == Set(["scope", "key", "uri", "modified", "total", "bytes", "status", "error", "taskID"]) })
        else { throw failure() }
        let saved = try JSONDecoder().decode([Job].self, from: data)
        guard saved.count <= 50 else { throw failure() }
        for job in saved {
          try validate(job)
          guard jobs[job.id] == nil else { throw failure() }
          jobs[job.id] = job
        }
      }
    } catch { jobs = [:]; storageError = true }
    session.getAllTasks { found in
      DispatchQueue.main.async {
        for case let task as URLSessionDownloadTask in found {
          guard let id = task.taskDescription, let job = self.jobs[id],
            job.taskID == task.taskIdentifier,
            ["queued", "downloading", "waiting"].contains(job.status)
          else { task.cancel(); continue }
          self.tasks[id] = task
          if task.state == .suspended { task.resume() }
        }
        for (id, var job) in self.jobs where self.tasks[id] == nil &&
          ["queued", "downloading", "waiting", "pausing"].contains(job.status) {
          job.status = "paused"
          job.error = "Download stopped. Resume to continue."
          self.jobs[id] = job
        }
        if !self.storageError {
          do { try self.persist() } catch { self.storageError = true }
        }
        self.restored = true
        let pending = self.waiting; self.waiting = []
        pending.forEach { $0() }
      }
    }
  }
  private func failure(_ message: String = "The saved transfer is invalid. Remove it and download again.") -> NSError {
    NSError(domain: "KinosailDownloads", code: 1, userInfo: [NSLocalizedDescriptionKey: message])
  }
  private func digest(_ value: String) -> Bool {
    value.range(of: "^[a-f0-9]{64}$", options: .regularExpression) != nil
  }
  private func validate(_ job: Job) throws {
    guard digest(job.scope), digest(job.key), job.uri.utf8.count <= 2048,
      let url = URL(string: job.uri), ["http", "https"].contains(url.scheme ?? ""),
      let host = url.host, url.scheme == "https" || approvedLocalHost(host),
      url.user == nil, url.password == nil, url.query == nil, url.fragment == nil,
      DownloadRecovery.validResource(url, etag: job.etag),
      job.total > 0, job.total <= limit, job.bytes >= 0, job.bytes <= job.total,
      job.modified.utf8.count == 29, job.modified.hasSuffix(" GMT"),
      Self.dateFormatter.date(from: job.modified).map({ Self.dateFormatter.string(from: $0) }) == job.modified,
      ["queued", "downloading", "waiting", "pausing", "paused", "complete"].contains(job.status),
      job.error.utf8.count <= 256, job.taskID >= 0,
      (0...5).contains(job.retries ?? 0),
      job.status != "complete" || job.bytes == job.total
    else { throw failure() }
  }
  private static let dateFormatter: DateFormatter = {
    let formatter = DateFormatter()
    formatter.locale = Locale(identifier: "en_US_POSIX")
    formatter.timeZone = TimeZone(secondsFromGMT: 0)
    formatter.dateFormat = "EEE, dd MMM yyyy HH:mm:ss 'GMT'"
    formatter.isLenient = false
    return formatter
  }()
  private func directory(_ job: Job) -> URL { root.appendingPathComponent(job.scope, isDirectory: true) }
  private func media(_ job: Job) -> URL { directory(job).appendingPathComponent(job.key + ".media") }
  private func resumeFile(_ job: Job) -> URL { directory(job).appendingPathComponent(job.key + ".resume") }
  private func prepare(_ directory: URL) throws {
    guard directory.resolvingSymlinksInPath().standardizedFileURL == directory.standardizedFileURL else { throw failure() }
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true,
      attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication])
    var url = directory
    var values = URLResourceValues(); values.isExcludedFromBackup = true
    try url.setResourceValues(values)
  }
  private func persist() throws {
    try prepare(root)
    try JSONEncoder().encode(Array(jobs.values)).write(to: root.appendingPathComponent("transfers.json"),
      options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
  }
  private func ready(_ promise: Promise, _ work: @escaping () throws -> Any?) {
    let run = {
      do { promise.resolve(try work()) }
      catch { promise.reject("DOWNLOAD_FAILED", error.localizedDescription) }
    }
    if restored { run() } else { waiting.append(run) }
  }
  func enqueue(_ raw: String, promise: Promise) {
    ready(promise) {
      guard !self.storageError, raw.utf8.count <= 8192,
        let data = raw.data(using: .utf8),
        let input = try JSONSerialization.jsonObject(with: data) as? [String: Any],
        Set(input.keys).subtracting(["etag"]) == Set(["scope", "key", "uri", "modified", "total", "authorization", "wifiOnly", "quota"]),
        let scope = input["scope"] as? String, let key = input["key"] as? String,
        let uri = input["uri"] as? String, let modified = input["modified"] as? String,
        let number = input["total"] as? NSNumber, CFGetTypeID(number) != CFBooleanGetTypeID(),
        number.doubleValue.isFinite, number.doubleValue.rounded() == number.doubleValue,
        number.doubleValue > 0, number.doubleValue <= Double(self.limit),
        let wifiValue = input["wifiOnly"] as? NSNumber, CFGetTypeID(wifiValue) == CFBooleanGetTypeID(),
        let quota = input["quota"] as? NSNumber, CFGetTypeID(quota) != CFBooleanGetTypeID(),
        quota.doubleValue.isFinite, quota.doubleValue.rounded() == quota.doubleValue,
        (quota.doubleValue == 0 || quota.doubleValue >= 1024 * 1024 * 1024), quota.doubleValue <= Double(self.limit),
        let authorization = input["authorization"] as? String,
        authorization.range(of: "^Bearer [^\\x00-\\x20\\x7f]{1,2048}$", options: .regularExpression) != nil
      else { throw self.failure() }
      var job = Job(scope: scope, key: key, uri: uri, modified: modified, total: number.int64Value)
      if input["etag"] != nil {
        guard let etag = input["etag"] as? String else { throw self.failure() }
        job.etag = etag
      }
      try self.validate(job)
      if let existing = self.jobs[job.id] {
        guard existing.uri == uri, existing.modified == modified, existing.total == job.total, existing.etag == job.etag else {
          throw self.failure("The original file changed. Remove this download and start again.")
        }
        if existing.status == "complete" || self.tasks[job.id] != nil { return nil }
        job = existing; job.status = "queued"; job.error = ""
      } else if self.jobs.count >= 50 { throw self.failure("Remove a download before adding another.") }
      job.retries = 0
      job.wifiOnly = wifiValue.boolValue
      let resumeURL = self.resumeFile(job)
      let resumeSize = (try? resumeURL.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0
      var resumeData: Data?
      if resumeSize > 0 && resumeSize <= 1024 * 1024,
        let sealed = try? Data(contentsOf: resumeURL),
        let box = try? AES.GCM.SealedBox(combined: sealed) {
        resumeData = try? AES.GCM.open(box, using: self.resumeKey(authorization), authenticating: self.resumeContext(job))
      }
      if resumeData == nil { job.bytes = 0 }
      let used = self.jobs.values.filter { $0.id != job.id }.reduce(Int64(0)) { $0 + $1.total }
      let space = try FileManager.default.attributesOfFileSystem(forPath: self.root.deletingLastPathComponent().path)[.systemFreeSize] as? NSNumber
      let reserved = self.jobs.values.filter { $0.id != job.id && $0.status != "complete" }.reduce(Int64(0)) { $0 + $1.total - $1.bytes }
      guard quota.int64Value == 0 || used + job.total <= quota.int64Value, let space,
        space.int64Value - reserved - self.reserve >= job.total - job.bytes
      else { throw self.failure("There is not enough space. Remove a download and try again.") }
      try self.prepare(self.directory(job))
      var request = URLRequest(url: URL(string: job.uri)!, cachePolicy: .reloadIgnoringLocalCacheData)
      request.allowsCellularAccess = !wifiValue.boolValue
      request.allowsExpensiveNetworkAccess = !wifiValue.boolValue
      request.setValue(authorization, forHTTPHeaderField: "Authorization")
      request.setValue("identity", forHTTPHeaderField: "Accept-Encoding")
      request.setValue(job.modified, forHTTPHeaderField: "If-Unmodified-Since")
      request.setValue(job.etag, forHTTPHeaderField: "If-Match")
      let task = resumeData.map { self.session.downloadTask(withResumeData: $0) } ?? self.session.downloadTask(with: request)
      job.taskID = task.taskIdentifier; task.taskDescription = job.id
      self.jobs[job.id] = job
      do { try self.persist() } catch { task.cancel(); self.jobs.removeValue(forKey: job.id); throw error }
      self.tasks[job.id] = task
      task.countOfBytesClientExpectsToReceive = job.total - job.bytes
      task.resume()
      return nil
    }
  }
  func snapshot(_ scope: String, promise: Promise) {
    ready(promise) {
      guard self.digest(scope), !self.storageError else { throw self.failure() }
      return self.jobs.values.filter { $0.scope == scope }.map {
        ["key": $0.key, "bytes": $0.bytes, "total": $0.total, "status": $0.status, "error": $0.error] as [String: Any]
      }
    }
  }
  func pause(_ scope: String, key: String, promise: Promise) {
    ready(promise) {
      guard self.digest(scope), self.digest(key) else { throw self.failure() }
      let id = scope + "/" + key
      guard var job = self.jobs[id], let task = self.tasks[id] else { return nil }
      job.status = "pausing"; self.jobs[id] = job; try self.persist()
      task.cancel { data in
        DispatchQueue.main.async {
          guard var current = self.jobs[id], current.taskID == task.taskIdentifier else { return }
          if !self.saveResume(data, job: current, task: task) { current.bytes = 0 }
          current.status = "paused"; current.error = ""; current.taskID = 0
          self.jobs[id] = current; self.tasks.removeValue(forKey: id); try? self.persist()
        }
      }
      return nil
    }
  }
  func remove(_ scope: String, key: String, promise: Promise) {
    ready(promise) {
      guard self.digest(scope), self.digest(key) else { throw self.failure() }
      let id = scope + "/" + key
      self.tasks.removeValue(forKey: id)?.cancel()
      if let job = self.jobs.removeValue(forKey: id) {
        for url in [self.media(job), self.resumeFile(job)] where FileManager.default.fileExists(atPath: url.path) {
          try FileManager.default.removeItem(at: url)
        }
      }
      try self.persist()
      return nil
    }
  }
  func clear(_ promise: Promise) {
    ready(promise) {
      self.tasks.values.forEach { $0.cancel() }
      self.tasks = [:]; self.jobs = [:]
      if FileManager.default.fileExists(atPath: self.root.path) { try FileManager.default.removeItem(at: self.root) }
      self.storageError = false
      return nil
    }
  }
  private func current(_ task: URLSessionTask) -> Job? {
    guard let id = task.taskDescription, let job = jobs[id], job.taskID == task.taskIdentifier else { return nil }
    return job
  }
  func urlSession(_ session: URLSession, taskIsWaitingForConnectivity task: URLSessionTask) {
    guard var job = current(task), job.status != "pausing" else { return }
    job.status = "waiting"; jobs[job.id] = job; try? persist()
  }
  func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask, didWriteData bytesWritten: Int64,
                  totalBytesWritten: Int64, totalBytesExpectedToWrite: Int64) {
    guard var job = current(downloadTask), job.status != "pausing" else { return }
    guard totalBytesWritten <= job.total, totalBytesExpectedToWrite <= job.total else { downloadTask.cancel(); return }
    job.bytes = max(0, totalBytesWritten); job.status = "downloading"; jobs[job.id] = job
  }
  func urlSession(_ session: URLSession, downloadTask: URLSessionDownloadTask, didFinishDownloadingTo location: URL) {
    guard var job = current(downloadTask), job.status != "pausing" else { return }
    do {
      guard let response = downloadTask.response as? HTTPURLResponse else { throw failure() }
      if ![200, 206].contains(response.statusCode) {
        job.status = "paused"
        switch response.statusCode {
        case 401, 403: job.error = "Access expired or downloads are no longer permitted. Sign in and try again."
        case 404, 410: job.error = "This title is no longer available on the Server."
        case 412, 416: job.error = "The original file changed. Remove this download and start again."
        default: job.error = "The Server could not send this download. Resume to try again."
        }
        jobs[job.id] = job
        try persist()
        return
      }
      guard response.url?.absoluteString == job.uri,
        response.value(forHTTPHeaderField: "Last-Modified") == job.modified,
        job.etag == nil || response.value(forHTTPHeaderField: "ETag") == job.etag,
        let size = try location.resourceValues(forKeys: [.fileSizeKey]).fileSize, Int64(size) == job.total
      else { throw failure() }
      try prepare(directory(job))
      let output = media(job)
      if FileManager.default.fileExists(atPath: output.path) { try FileManager.default.removeItem(at: output) }
      try FileManager.default.moveItem(at: location, to: output)
      try FileManager.default.setAttributes([.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication], ofItemAtPath: output.path)
      job.bytes = job.total; job.status = "complete"; job.error = ""
      try? FileManager.default.removeItem(at: resumeFile(job))
    } catch { job.status = "paused"; job.error = "The download could not be verified. Remove it and try again." }
    jobs[job.id] = job
    do { try persist() } catch { storageError = true }
  }
  func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
    guard var job = current(task), job.status != "pausing" else { return }
    tasks.removeValue(forKey: job.id)
    if job.status != "complete" {
      job.status = "paused"
      if job.error.isEmpty { job.error = "Download interrupted. Resume to try again." }
      let data = (error as NSError?)?.userInfo[NSURLSessionDownloadTaskResumeData] as? Data
      let resumable = saveResume(data, job: job, task: task)
      if !resumable { job.bytes = 0 }
      let response = task.response as? HTTPURLResponse
      if let delay = DownloadRecovery.delay(error: error as NSError?, response: response, attempt: job.retries ?? 0),
        var request = task.originalRequest, request.url?.absoluteString == job.uri {
        request.setValue(nil, forHTTPHeaderField: "Range")
        request.setValue(nil, forHTTPHeaderField: "If-Range")
        job.retries = (job.retries ?? 0) + 1
        let next: URLSessionDownloadTask
        if resumable, let data { next = session.downloadTask(withResumeData: data) }
        else { next = session.downloadTask(with: request) }
        next.taskDescription = job.id
        next.earliestBeginDate = Date(timeIntervalSinceNow: delay + Double.random(in: 0...1))
        next.countOfBytesClientExpectsToReceive = job.total - job.bytes
        job.taskID = next.taskIdentifier; job.status = "waiting"
        job.error = (response?.statusCode ?? 0) >= 400 ? "The Server is busy. Retrying automatically." : ""
        jobs[job.id] = job
        do {
          try persist()
          tasks[job.id] = next
          next.resume()
          return
        } catch {
          next.cancel()
          job.status = "paused"; job.error = "Download recovery could not be saved. Resume to try again."
        }
      }
      jobs[job.id] = job; try? persist()
    }
  }
  private func resumeKey(_ authorization: String) -> SymmetricKey {
    SymmetricKey(data: SHA256.hash(data: Data(authorization.utf8)))
  }
  private func resumeContext(_ job: Job) -> Data {
    Data((job.id + "\n" + job.uri + "\n" + job.modified + "\n" + String(job.total) + "\n" + String(job.wifiOnly ?? true) + (job.etag.map { "\n" + $0 } ?? "")).utf8)
  }
  @discardableResult
  private func saveResume(_ data: Data?, job: Job, task: URLSessionTask) -> Bool {
    // Resume blobs contain authenticated requests. Seal them to the current Viewer
    // credential and exact resource so imported/tampered files cannot change requests.
    guard let data, data.count <= 1024 * 1024 - 64,
      let authorization = task.originalRequest?.value(forHTTPHeaderField: "Authorization"),
      let sealed = try? AES.GCM.seal(data, using: resumeKey(authorization), authenticating: resumeContext(job)),
      let combined = sealed.combined else {
      try? FileManager.default.removeItem(at: resumeFile(job)); return false
    }
    do {
      try combined.write(to: resumeFile(job), options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
      return true
    } catch { try? FileManager.default.removeItem(at: resumeFile(job)); return false }
  }
  func urlSessionDidFinishEvents(forBackgroundURLSession session: URLSession) {
    let finished = completion; completion = nil; finished?()
  }
}

#endif

import ExpoModulesCore
import UIKit

public final class DownloadsAppDelegateSubscriber: ExpoAppDelegateSubscriber {
  public func application(_ application: UIApplication, handleEventsForBackgroundURLSession identifier: String,
                          completionHandler: @escaping () -> Void) {
    #if os(iOS)
    if identifier == VerifiedDownloads.identifier { VerifiedDownloads.shared.completion = completionHandler; return }
    guard identifier == BackgroundDownloads.identifier else { completionHandler(); return }
    BackgroundDownloads.shared.completion = completionHandler
    #else
    completionHandler()
    #endif
  }
}
