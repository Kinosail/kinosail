import ExpoModulesCore
import Foundation
import Network

public final class ProtectedMediaModule: Module {
  private var gateway: ProtectedMediaGateway?
  private var generation = 0
  private let gatewayLock = NSLock()
  public func definition() -> ModuleDefinition {
    Name("ProtectedMedia")
    AsyncFunction("open") { (uri: String, authorization: String, id: Int) async throws -> String in
      guard id > 0 else { throw MediaTransportError.invalidSource }
      let local = approvedOfflineURL(uri)
      let next = local == nil ? try ProtectedMediaGateway(uri: uri, authorization: authorization) : nil
      try self.gatewayLock.withLock {
        guard id > self.generation else { next?.close(); throw MediaTransportError.unavailable }
        self.generation = id
        self.gateway?.close()
        self.gateway = next
      }
      if let local { return local.absoluteString }
      guard let next else { throw MediaTransportError.invalidSource }
      return try await next.start()
    }
    Function("close") { (id: Int) in
      self.gatewayLock.withLock {
        if id >= self.generation {
          self.generation = id
          self.gateway?.close()
          self.gateway = nil
        }
      }
    }
    AsyncFunction("prepareOfflineStorage") {
      guard let documents = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first else { throw MediaTransportError.unavailable }
      var root = documents.appendingPathComponent("kinosail-offline", isDirectory: true)
      try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
      var values = URLResourceValues()
      values.isExcludedFromBackup = true
      try root.setResourceValues(values)
    }
    #if os(iOS)
    AsyncFunction("authorizeVerifiedDownloads") { (scope: String, authorization: String, promise: Promise) in
      VerifiedDownloads.shared.authorize(scope, authorization: authorization, promise: promise)
    }
    AsyncFunction("enqueuePreparingDownload") { (raw: String,promise: Promise) in
      VerifiedDownloads.shared.enqueuePreparation(raw,promise:promise)
    }
    AsyncFunction("enqueueVerifiedDownload") { (raw: String, promise: Promise) in
      VerifiedDownloads.shared.enqueue(raw, promise: promise)
    }
    AsyncFunction("verifiedDownloadSnapshot") { (scope: String, promise: Promise) in
      VerifiedDownloads.shared.snapshot(scope, promise: promise)
    }
    AsyncFunction("pauseVerifiedDownload") { (scope: String, key: String, promise: Promise) in
      VerifiedDownloads.shared.pause(scope, key: key, promise: promise)
    }
    AsyncFunction("removeVerifiedDownload") { (scope: String, key: String, promise: Promise) in
      VerifiedDownloads.shared.remove(scope, key: key, promise: promise)
    }
    AsyncFunction("clearVerifiedDownloads") { (promise: Promise) in
      VerifiedDownloads.shared.clear(promise)
    }
    AsyncFunction("checkVerifiedDownload") { (scope: String, key: String, promise: Promise) in
      VerifiedDownloads.shared.check(scope, key: key, promise: promise)
    }
    AsyncFunction("setDownloadPlaybackActive") { (active: Bool) in
      VerifiedDownloads.shared.setPlayback(active)
    }
    AsyncFunction("enqueueDownload") { (raw: String, promise: Promise) in
      BackgroundDownloads.shared.enqueue(raw, promise: promise)
    }.runOnQueue(.main)
    AsyncFunction("downloadSnapshot") { (scope: String, promise: Promise) in
      BackgroundDownloads.shared.snapshot(scope, promise: promise)
    }.runOnQueue(.main)
    AsyncFunction("pauseDownload") { (scope: String, key: String, promise: Promise) in
      BackgroundDownloads.shared.pause(scope, key: key, promise: promise)
    }.runOnQueue(.main)
    AsyncFunction("removeDownload") { (scope: String, key: String, promise: Promise) in
      BackgroundDownloads.shared.remove(scope, key: key, promise: promise)
    }.runOnQueue(.main)
    AsyncFunction("clearDownloads") { (promise: Promise) in
      BackgroundDownloads.shared.clear(promise)
    }.runOnQueue(.main)
    #endif
    OnDestroy { self.gateway?.close() }
    View(LocalVideoView.self) {
      Events("onPlaying", "onPaused", "onFirstFrame", "onBuffering", "onPictureInPictureReady", "onProgress", "onEnd", "onError", "onTracks")
      Prop("rate") { (view: LocalVideoView, rate: Double) in view.setRate(rate) }
      Prop("nightMode") { (view: LocalVideoView, value: Bool) in view.nightMode = value }
      Prop("dialogueBoost") { (view: LocalVideoView, value: Bool) in view.dialogueBoost = value }
      Prop("volumeBoost") { (view: LocalVideoView, value: Double) in view.volumeBoost = value }
      Prop("sleepDeadline") { (view: LocalVideoView, value: Double) in view.sleepDeadline = value }
      Prop("sleepPosition") { (view: LocalVideoView, value: Double) in view.sleepPosition = value.isFinite && (0...31_536_000).contains(value) ? value : 0 }
      Prop("audioOnly") { (view: LocalVideoView, value: Bool) in view.audioOnly = value }
      Prop("uri") { (view: LocalVideoView, uri: String) in view.sourceURI = uri }
      Prop("start") { (view: LocalVideoView, start: Double) in view.startSeconds = start }
      OnViewDidUpdateProps { (view: LocalVideoView) in view.load(view.sourceURI) }
      Prop("paused") { (view: LocalVideoView, paused: Bool) in view.setPaused(paused) }
      AsyncFunction("startPictureInPicture") { (view: LocalVideoView) in view.startPictureInPicture() }
      AsyncFunction("seek") { (view: LocalVideoView, seconds: Double) in view.seek(seconds) }
      AsyncFunction("getTracks") { (view: LocalVideoView) in view.tracks() }
      AsyncFunction("selectAudioTrack") { (view: LocalVideoView, index: Int) in view.selectAudio(index) }
      AsyncFunction("selectSubtitleTrack") { (view: LocalVideoView, index: Int) in view.selectSubtitle(index) }
      AsyncFunction("setAudioDelay") { (view: LocalVideoView, amount: Int) in view.delay(amount, audio: true) }
      AsyncFunction("setSubtitleDelay") { (view: LocalVideoView, amount: Int) in view.delay(amount, audio: false) }
    }
  }
}

private enum MediaTransportError: Error { case invalidSource, unavailable }

// One active source, loopback only, with a fresh capability for each playback.
// URLSession owns authentication and TLS. VLC sees only an opaque local URL.
private final class ProtectedMediaGateway {
  private let queue = DispatchQueue(label: "com.kinosail.player.media")
  private let listener: NWListener
  private let transfer: MediaTransferSession
  private let source: URL
  private let authorization: String
  private let path = "/" + UUID().uuidString + UUID().uuidString
  private var startResolved = false
  private var streams: [UUID: MediaConnection] = [:]

  init(uri: String, authorization: String) throws {
    guard uri.utf8.count <= 2048, let url = URL(string: uri),
      ["http", "https"].contains(url.scheme ?? ""), url.host != nil,
      url.user == nil, url.password == nil, url.fragment == nil,
      url.path.range(of: "^/media/[A-Za-z0-9_-]{1,128}$", options: .regularExpression) != nil, url.query == nil,
      url.scheme == "https" || approvedLocalHost(url.host ?? ""),
      authorization.hasPrefix("Bearer "), authorization.utf8.count > 7,
      authorization.utf8.count <= 2055,
      !authorization.unicodeScalars.contains(where: { CharacterSet.controlCharacters.contains($0) })
    else { throw MediaTransportError.invalidSource }
    source = url
    self.authorization = authorization
    let parameters = NWParameters.tcp
    parameters.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: .any)
    listener = try NWListener(using: parameters)
    transfer = MediaTransferSession(queue: queue)
    listener.newConnectionHandler = { [weak self] connection in
      guard let self, self.streams.count < 4 else { connection.cancel(); return }
      let id = UUID()
      let stream = MediaConnection(connection: connection, queue: self.queue,
        path: self.path, source: self.source, authorization: self.authorization, transfer: self.transfer) { [weak self] in
          self?.queue.async { self?.streams.removeValue(forKey: id) }
        }
      self.streams[id] = stream
      stream.start()
    }
  }

  func start() async throws -> String {
    try await withCheckedThrowingContinuation { continuation in
      listener.stateUpdateHandler = { [weak self] state in
        guard let self, !self.startResolved else { return }
        switch state {
        case .ready:
          self.startResolved = true
          guard let port = self.listener.port else {
            continuation.resume(throwing: MediaTransportError.unavailable); return
          }
          continuation.resume(returning: "http://127.0.0.1:\(port.rawValue)\(self.path)")
        case .failed, .cancelled:
          self.startResolved = true
          continuation.resume(throwing: MediaTransportError.unavailable)
        default: break
        }
      }
      listener.start(queue: queue)
    }
  }

  func close() {
    listener.cancel()
    queue.async { [self] in
      let active = Array(streams.values)
      streams.removeAll()
      active.forEach { $0.close() }
      transfer.close()
    }
  }
  deinit { listener.cancel(); transfer.close() }
}

private final class MediaConnection: MediaTransferConsumer {
  private let connection: NWConnection
  private let queue: DispatchQueue
  private let path: String
  private let source: URL
  private let authorization: String
  private let finished: () -> Void
  private var header = Data()
  private var task: URLSessionDataTask?
  private let transfer: MediaTransferSession
  private var closed = false

  init(connection: NWConnection, queue: DispatchQueue, path: String, source: URL,
       authorization: String, transfer: MediaTransferSession, finished: @escaping () -> Void) {
    self.connection = connection; self.queue = queue; self.path = path
    self.source = source; self.authorization = authorization; self.finished = finished
    self.transfer = transfer
  }
  func start() {
    connection.stateUpdateHandler = { [weak self] state in
      if case .failed = state { self?.close() }
      if case .cancelled = state { self?.close() }
    }
    connection.start(queue: queue)
    queue.asyncAfter(deadline: .now() + 10) { [weak self] in
      if self?.task == nil { self?.close() }
    }
    receiveHeader()
  }
  private func receiveHeader() {
    connection.receive(minimumIncompleteLength: 1, maximumLength: 8192 - header.count) { [weak self] data, _, complete, error in
      guard let self, !self.closed else { return }
      if let data { self.header.append(data) }
      if let range = self.header.range(of: Data("\r\n\r\n".utf8)) {
        guard range.upperBound == self.header.count else { self.close(); return }
        self.forward()
      } else if complete || error != nil || self.header.count >= 8192 { self.close() }
      else { self.receiveHeader() }
    }
  }
  private func forward() {
    guard let raw = String(data: header, encoding: .utf8) else { close(); return }
    let lines = raw.components(separatedBy: "\r\n")
    let first = (lines.first ?? "").split(separator: " ", omittingEmptySubsequences: false)
    guard first.count == 3, ["GET", "HEAD"].contains(String(first[0])),
      first[1] == path, first[2] == "HTTP/1.1" || first[2] == "HTTP/1.0"
    else { close(); return }
    var request = URLRequest(url: source, cachePolicy: .reloadIgnoringLocalCacheData, timeoutInterval: 30)
    request.httpMethod = String(first[0])
    request.setValue(authorization, forHTTPHeaderField: "Authorization")
    request.setValue("identity", forHTTPHeaderField: "Accept-Encoding")
    var names = Set<String>()
    for line in lines.dropFirst() where !line.isEmpty {
      guard let colon = line.firstIndex(of: ":") else { close(); return }
      let name = String(line[..<colon]).lowercased()
      let value = line[line.index(after: colon)...].trimmingCharacters(in: .whitespaces)
      guard names.insert(name).inserted, name != "transfer-encoding",
        name != "content-length", !value.contains("\r"), !value.contains("\n")
      else { close(); return }
      if name == "range" {
        guard value.count <= 64, value.range(of: "^bytes=([0-9]{1,19}-[0-9]{0,19}|-[0-9]{1,19})$", options: .regularExpression) != nil
        else { close(); return }
        let bounds = String(value.dropFirst(6)).split(separator: "-", omittingEmptySubsequences: false)
        guard bounds.count == 2 else { close(); return }
        if bounds[0].isEmpty {
          guard let suffix = UInt64(bounds[1]), suffix > 0 else { close(); return }
        } else {
          guard let start = UInt64(bounds[0]) else { close(); return }
          if !bounds[1].isEmpty {
            guard let end = UInt64(bounds[1]), end >= start else { close(); return }
          }
        }
        request.setValue(value, forHTTPHeaderField: "Range")
      }
    }
    task = transfer.dataTask(with: request, consumer: self)
    guard task != nil else { close(); return }
    task?.resume()
  }
  func receive(_ response: URLResponse, completion: @escaping (URLSession.ResponseDisposition) -> Void) {
    guard let http = response as? HTTPURLResponse, [200, 206, 416].contains(http.statusCode) else {
      completion(.cancel); close(); return
    }
    var output = "HTTP/1.1 \(http.statusCode) Media\r\nConnection: close\r\nCache-Control: no-store\r\n"
    for name in ["Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"] {
      if let value = http.value(forHTTPHeaderField: name), value.count <= 512,
        !value.contains("\r"), !value.contains("\n") { output += "\(name): \(value)\r\n" }
    }
    connection.send(content: Data((output + "\r\n").utf8), completion: .contentProcessed { [weak self] error in
      if error != nil { self?.close() }
    })
    completion(.allow)
  }
  func receive(_ data: Data, task: URLSessionDataTask) {
    guard !closed else { return }
    task.suspend()
    connection.send(content: data, completion: .contentProcessed { [weak self] error in
      if error != nil { self?.close() } else { task.resume() }
    })
  }
  func complete(_ error: Error?) {
    guard !closed else { return }
    guard error == nil else { close(); return }
    connection.send(content: nil, isComplete: true, completion: .contentProcessed { [weak self] _ in self?.close() })
  }
  func close() {
    guard !closed else { return }
    closed = true; connection.cancel(); transfer.cancel(task); finished()
  }
}

// Only completed media paths inside this application's offline store are accepted.
func approvedOfflineURL(_ uri: String) -> URL? {
  guard uri.utf8.count <= 2048, let url = URL(string: uri), url.isFileURL,
    url.host == nil || url.host == "", url.query == nil, url.fragment == nil,
    let documents = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first
  else { return nil }
  let root = documents.appendingPathComponent("kinosail-offline").resolvingSymlinksInPath().path + "/"
  let resolved = url.resolvingSymlinksInPath().standardizedFileURL
  guard resolved.path.hasPrefix(root) else { return nil }
  let relative = String(resolved.path.dropFirst(root.count))
  guard relative.range(of: "^[a-f0-9]{64}/[a-f0-9]{64}\\.media$", options: .regularExpression) != nil,
    let values = try? resolved.resourceValues(forKeys: [.isRegularFileKey]), values.isRegularFile == true
  else { return nil }
  return resolved
}

func approvedLocalHost(_ input: String) -> Bool {
  let host = input.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "[]"))
  if host == "localhost" || host.hasSuffix(".local") || host == "::1" || host.hasPrefix("fe80:") { return true }
  if host.range(of: "^f[cd][0-9a-f]{2}:", options: .regularExpression) != nil { return true }
  let parts = host.split(separator: ".", omittingEmptySubsequences: false)
  guard parts.count == 4, parts.allSatisfy({ UInt8($0) != nil }) else { return false }
  let numbers = parts.compactMap { UInt8($0) }
  return numbers[0] == 10 || numbers[0] == 127 || numbers[0] == 169 && numbers[1] == 254 ||
    numbers[0] == 172 && (16...31).contains(numbers[1]) || numbers[0] == 192 && numbers[1] == 168
}
