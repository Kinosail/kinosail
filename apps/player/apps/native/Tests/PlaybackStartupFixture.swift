import Foundation
import Network
import Synchronization
import Testing
@testable import KinosailPlayer

final class PlaybackStartupFixture: @unchecked Sendable {
    struct ProgressReceipt: Sendable {
        let itemID: String; let seconds: Double; let watched: Bool; let revision: Int
    }
    struct State {
        var counts: [String: Int] = [:]; var delayed = false; var denied = true; var media: Data?
        var savedSeconds: Double?; var holdPreferences = false; var denialStatus = 403
        var hlsResources: [String: Data] = [:]; var playbackResponse: Data?
        var images: [String: Data] = [:]; var serverProgress: [String: WatchProgress]?; var missingItems: Set<String> = []; var conflicts = 0
        var progressReceipts: [ProgressReceipt] = []
        var progressSafetyMediaStatus: Int?
        var progressSafetyHTTPReceipts: [PlaybackFailureHTTPReceipt] = []
        var progressSafetyDroppedHTTPReceipts = 0
    }
    let state = Mutex(State())
    let listener: NWListener
    let server: ServerAddress
    let client: ServerClient
    let viewer: Viewer
    private let queue = DispatchQueue(label: "playback-startup-fixture")
    private let heldMedia = Mutex<(holding: Bool, pending: [(NWConnection, Data)])>((false, []))
    private let heldPreferences = Mutex<[(NWConnection, Data)]>([])
    private let progressSafetyConnections = Mutex<[ObjectIdentifier: NWConnection]>([:])
    private let progressSafetyListenerCancelled = Mutex(false)
    private let progressSafetyOrigin = ProcessInfo.processInfo.systemUptime
    var progressSafetyMediaStatus: Int? { get { state.withLock { $0.progressSafetyMediaStatus } } set { state.withLock { $0.progressSafetyMediaStatus = newValue } } }
    var progressSafetyHTTPReceipts: [PlaybackFailureHTTPReceipt] { state.withLock { $0.progressSafetyHTTPReceipts } }
    var progressSafetyDroppedHTTPReceipts: Int { state.withLock { $0.progressSafetyDroppedHTTPReceipts } }
    var holdMedia: Bool { get { heldMedia.withLock { $0.holding } } set { heldMedia.withLock { $0.holding = newValue } } }
    var heldMediaCount: Int { heldMedia.withLock { $0.pending.count } }
    func releaseMedia() {
        let held = heldMedia.withLock { gate in
            gate.holding = false
            let result = gate.pending; gate.pending.removeAll(); return result
        }
        for (connection, response) in held {
            connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
        }
    }
    private func deliverMedia(_ response: Data, connection: NWConnection) {
        let held = heldMedia.withLock { gate in
            guard gate.holding else { return false }
            gate.pending.append((connection, response)); return true
        }
        guard !held else { return }
        connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
    }
    var holdPreferences: Bool { get { state.withLock { $0.holdPreferences } } set { state.withLock { $0.holdPreferences = newValue } } }
    var denialStatus: Int { get { state.withLock { $0.denialStatus } } set { state.withLock { $0.denialStatus = newValue } } }
    var heldPreferenceCount: Int { heldPreferences.withLock { $0.count } }
    func releasePreferences() {
        let held = heldPreferences.withLock { pending in
            let result = pending; pending.removeAll(); return result
        }
        for (connection, response) in held {
            connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
        }
    }
    var delayed: Bool { get { state.withLock { $0.delayed } } set { state.withLock { $0.delayed = newValue } } }
    var denied: Bool { get { state.withLock { $0.denied } } set { state.withLock { $0.denied = newValue } } }
    var media: Data? { get { state.withLock { $0.media } } set { state.withLock { $0.media = newValue } } }
    var hlsResources: [String: Data] { get { state.withLock { $0.hlsResources } } set { state.withLock { $0.hlsResources = newValue } } }
    var playbackResponse: Data? { get { state.withLock { $0.playbackResponse } } set { state.withLock { $0.playbackResponse = newValue } } }
    var images: [String: Data] { get { state.withLock { $0.images } } set { state.withLock { $0.images = newValue } } }
    var serverProgress: [String: WatchProgress]? { get { state.withLock { $0.serverProgress } } set { state.withLock { $0.serverProgress = newValue } } }
    var missingItems: Set<String> { get { state.withLock { $0.missingItems } } set { state.withLock { $0.missingItems = newValue } } }
    var conflicts: Int { state.withLock { $0.conflicts } }
    func count(_ path: String) -> Int { state.withLock { $0.counts[path] ?? 0 } }
    var savedSeconds: Double? { state.withLock { $0.savedSeconds } }
    var progressReceipts: [ProgressReceipt] { state.withLock { $0.progressReceipts } }

    init() async throws {
        let parameters = NWParameters.tcp
        parameters.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: .any)
        listener = try NWListener(using: parameters)
        let ready = Mutex(false)
        listener.stateUpdateHandler = { if case .ready = $0 { ready.withLock { $0 = true } } }
        listener.newConnectionHandler = { $0.cancel() }
        listener.start(queue: queue)
        for _ in 0..<500 where !ready.withLock({ $0 }) { try await Task.sleep(for: .milliseconds(10)) }
        try #require(ready.withLock { $0 })
        server = try ServerAddress("http://127.0.0.1:\(try #require(listener.port).rawValue)")
        viewer = try Viewer(.object(["server": .string("Isolated startup"), "serverId": .string("startup-fixture"),
            "viewer": .object(["id": .string("fixture"), "name": .string("Fixture"), "owner": .bool(false),
                               "downloads": .bool(false), "transcode": .bool(false), "remote": .bool(false)])]))
        client = try ServerClient(server: server, token: "fictional-startup-token", viewer: viewer)
        listener.stateUpdateHandler = { [weak self] state in
            if case .cancelled = state { self?.progressSafetyListenerCancelled.withLock { $0 = true } }
        }
        listener.newConnectionHandler = { [weak self] connection in
            guard let self else { connection.cancel(); return }
            let identity = ObjectIdentifier(connection)
            self.progressSafetyConnections.withLock { $0[identity] = connection }
            connection.stateUpdateHandler = { [weak self] state in
                if case .cancelled = state { self?.progressSafetyConnections.withLock { $0.removeValue(forKey: identity) } }
            }
            connection.start(queue: self.queue)
            self.receive(connection, data: Data())
        }
    }
    func close() { listener.cancel() }
    func progressSafetyCloseAndJoin() async throws {
        close()
        let connections = progressSafetyConnections.withLock { Array($0.values) }
        connections.forEach { $0.cancel() }
        let deadline = ContinuousClock.now.advanced(by: .seconds(3))
        while ContinuousClock.now < deadline {
            if progressSafetyListenerCancelled.withLock({ $0 }) && progressSafetyConnections.withLock({ $0.isEmpty }) {
                print("PLAYBACK_PROGRESS_SAFETY_FIXTURE_CLEANUP listenerCancelled=true activeConnections=0")
                return
            }
            try await Task.sleep(for: .milliseconds(10))
        }
        try #require(false, "Fixture listener/connections did not close inside3s")
    }
    func item(_ id: String) throws -> MediaItem {
        try MediaItem(.object(["id": .string(id), "kind": .string("video"), "title": .string("Fixture")]), server: server)
    }
    private func send(_ data: Data, type: String, head: Bool = false, connection: NWConnection) {
        var response = Data("HTTP/1.1 200 OK\r\nContent-Type: \(type)\r\nContent-Length: \(data.count)\r\nConnection: close\r\n\r\n".utf8)
        if !head { response.append(data) }
        connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
    }
    private func recordProgressSafety(_ operation: PlaybackFailureHTTPReceipt.Operation, status: Int, header: String) {
        let values = header.components(separatedBy: "\r\n").dropFirst().compactMap { line -> String? in
            guard let colon = line.firstIndex(of: ":"),
                  line[..<colon].lowercased() == "x-request-id" else { return nil }
            return String(line[line.index(after: colon)...]).trimmingCharacters(in: .whitespaces)
        }
        guard values.count <= 1,
              values.first.map({ PlaybackFailureHTTPReceipt.requestUUID($0) != nil }) ?? true,
              operation == .media || values.count == 1 else {
            state.withLock { $0.progressSafetyDroppedHTTPReceipts += 1 }; return
        }
        let requestID = values.first
        let receipt = PlaybackFailureHTTPReceipt(operation: operation, requestID: requestID,
            status: status, elapsed: ProcessInfo.processInfo.systemUptime - progressSafetyOrigin)
        state.withLock { state in
            guard let receipt, state.progressSafetyHTTPReceipts.count < 64 else {
                state.progressSafetyDroppedHTTPReceipts += 1; return
            }
            state.progressSafetyHTTPReceipts.append(receipt)
        }
    }
    private func sendMedia(_ media: Data, header: String, head: Bool, connection: NWConnection) {
        let range = header.components(separatedBy: "\r\n").first { $0.lowercased().hasPrefix("range:") }
            .map { $0.dropFirst(6).trimmingCharacters(in: .whitespaces) }
        var lower = 0, upper = media.count - 1
        if let range {
            guard (try? MediaRange.validate(range)) != nil else { connection.cancel(); return }
            let bounds = range.dropFirst(6).split(separator: "-", omittingEmptySubsequences: false)
            if bounds[0].isEmpty { lower = max(0, media.count - (Int(bounds[1]) ?? media.count)) }
            else { lower = Int(bounds[0]) ?? media.count; upper = min(upper, Int(bounds[1]) ?? upper) }
        }
        guard lower <= upper, upper < media.count else { connection.cancel(); return }
        let body = media.subdata(in: lower..<(upper + 1))
        let status = range == nil ? "200 OK" : "206 Partial Content"
        recordProgressSafety(.media, status: range == nil ? 200 : 206, header: header)
        let contentRange = range == nil ? "" : "Content-Range: bytes \(lower)-\(upper)/\(media.count)\r\n"
        var response = Data("HTTP/1.1 \(status)\r\nContent-Type: video/mp4\r\nAccept-Ranges: bytes\r\nContent-Length: \(body.count)\r\n\(contentRange)Connection: close\r\n\r\n".utf8)
        if !head { response.append(body) }
        deliverMedia(response, connection: connection)
    }
    private func receive(_ connection: NWConnection, data: Data) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 8192) { [weak self] chunk, _, complete, error in
            guard let self, error == nil, let chunk, data.count + chunk.count <= 8192 else { connection.cancel(); return }
            let next = data + chunk
            guard let boundary = next.range(of: Data("\r\n\r\n".utf8)),
                  let header = String(data: next[..<boundary.lowerBound], encoding: .utf8) else {
                if complete { connection.cancel() } else { self.receive(connection, data: next) }
                return
            }
            let parts = header.components(separatedBy: "\r\n")[0].split(separator: " ")
            guard parts.count == 3, ["GET", "HEAD", "PUT"].contains(String(parts[0])), let url = URL(string: String(parts[1]), relativeTo: self.server.url) else { connection.cancel(); return }
            let path = url.path
            let length = header.components(separatedBy: "\r\n").first { $0.lowercased().hasPrefix("content-length:") }
                .flatMap { Int($0.split(separator: ":").last!.trimmingCharacters(in: .whitespaces)) } ?? 0
            guard (0...4096).contains(length) else { connection.cancel(); return }
            if next.count - boundary.upperBound < length {
                if complete { connection.cancel() } else { self.receive(connection, data: next) }
                return
            }
            let (count, delayed, denied) = self.state.withLock { state in
                state.counts[path, default: 0] += 1
                return (state.counts[path]!, state.delayed, state.denied)
            }
            if let image = self.images[path] { self.send(image, type: "image/png", head: parts[0] == "HEAD", connection: connection); return }
            if let resource = self.hlsResources[path] {
                self.send(resource, type: path.hasSuffix(".m3u8") ? "application/vnd.apple.mpegurl" : "video/mp2t",
                          head: parts[0] == "HEAD", connection: connection)
                return
            }
            if path.hasPrefix("/media/"), let status = self.progressSafetyMediaStatus {
                guard status == 403 else { connection.cancel(); return }
                self.recordProgressSafety(.media, status: status, header: header)
                let response = Data("HTTP/1.1 403 Fixture denial\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".utf8)
                self.deliverMedia(response, connection: connection)
                return
            }
            if path.hasPrefix("/media/"), let media = self.media {
                self.sendMedia(media, header: header, head: parts[0] == "HEAD", connection: connection)
                return
            }
            if path == "/api/v1/me", !denied {
                self.send(try! JSONEncoder().encode(self.viewer.json), type: "application/json", connection: connection)
                return
            }
            let segments = path.split(separator: "/").map(String.init)
            if segments.count >= 4, segments[0] == "api", segments[2] == "items", self.missingItems.contains(segments[3]) {
                let response = Data("HTTP/1.1 404 Not Found\r\nContent-Type: application/json\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}".utf8)
                connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
                return
            }
            if segments.count == 4, segments[0] == "api", segments[2] == "items", parts[0] == "GET", let progress = self.serverProgress {
                let current = progress[segments[3]] ?? WatchProgress()
                let body = JSONValue.object(["item": .object(["id": .string(segments[3]), "kind": .string("video"), "title": .string("Fixture"), "progress": current.json]),
                                             "listed": .bool(false), "profileId": .string("fixture")])
                self.send(try! JSONEncoder().encode(body), type: "application/json", connection: connection)
                return
            }
            if path.hasSuffix("/progress/sync"), self.serverProgress != nil {
                guard let raw = try? StrictJSON.decode(Data(next[boundary.upperBound..<(boundary.upperBound + length)])),
                      let body = try? raw.object(allowing: ["progress", "expected", "playbackToken"]),
                      let update = try? WatchProgress(body["progress"] ?? .null), let expected = try? WatchProgress(body["expected"] ?? .null),
                      segments.count == 6 else { connection.cancel(); return }
                let itemID = segments[3]
                let (status, result): (Int, WatchProgress) = self.state.withLock { state in
                    let current = state.serverProgress?[itemID] ?? WatchProgress()
                    if current.session == update.session, current.revision >= update.revision { return (200, current) }
                    if current.session != update.session, current.session != expected.session || current.revision != expected.revision
                        || current.seconds != expected.seconds || current.watched != expected.watched {
                        state.conflicts += 1
                        return (409, current)
                    }
                    var saved = update; saved.dismissed = false
                    state.serverProgress?[itemID] = saved
                    state.savedSeconds = saved.seconds
                    state.progressReceipts.append(ProgressReceipt(itemID: itemID, seconds: saved.seconds, watched: saved.watched, revision: saved.revision))
                    return (200, saved)
                }
                let payload: JSONValue = status == 409 ? .object(["error": .string("Progress changed on another device."), "progress": result.synchronizationJSON]) : result.synchronizationJSON
                let data = try! JSONEncoder().encode(payload)
                let response = Data("HTTP/1.1 \(status) Fixture\r\nContent-Type: application/json\r\nContent-Length: \(data.count)\r\nConnection: close\r\n\r\n".utf8) + data
                connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
                return
            }
            if path.hasSuffix("/progress/sync") {
                guard let raw = try? StrictJSON.decode(Data(next[boundary.upperBound..<(boundary.upperBound + length)])),
                      let body = try? raw.object(allowing: ["progress", "expected", "playbackToken"]),
                      let progress = body["progress"], let data = try? JSONEncoder().encode(progress) else { connection.cancel(); return }
                guard let saved = try? WatchProgress(progress),
                      let itemID = try? Input.id(String(path.split(separator: "/").dropFirst(3).first ?? "")) else { connection.cancel(); return }
                let accepted = self.state.withLock { state in
                    guard state.progressReceipts.count < 64 else { return false }
                    state.savedSeconds = saved.seconds
                    state.progressReceipts.append(ProgressReceipt(itemID: itemID, seconds: saved.seconds, watched: saved.watched, revision: saved.revision))
                    return true
                }
                guard accepted else { connection.cancel(); return }
                self.recordProgressSafety(.progressSync, status: 200, header: header)
                self.send(data, type: "application/json", connection: connection)
                return
            }
            if path.hasSuffix("/playback"), !denied, let response = self.playbackResponse {
                self.recordProgressSafety(.itemsPlayback, status: 200, header: header)
                self.send(response, type: "application/json", connection: connection)
                return
            }
            let id = path.split(separator: "/").dropFirst(3).first.map(String.init) ?? "movie"
            let preferences = path.hasSuffix("/playback-preferences")
            let preferencesBody = String(decoding: try! JSONEncoder().encode(JSONValue.object(["playback": PlaybackPreferences().json, "overridden": .bool(false)])), as: UTF8.self)
            let duration = self.media == nil ? 60 : 12
            let start = self.media == nil ? 0 : 3
            let body = preferences ? preferencesBody : "{\"media\":{\"duration\":\(duration)},\"plan\":{\"allowed\":true,\"mode\":\"direct\",\"reason\":\"direct-preferred\"},\"duration\":\(duration),\"start\":\(start),\"directAllowed\":true,\"direct\":\"/media/\(id)\",\"directType\":\"video/mp4\"}"
            let payload = Data((denied ? "{\"error\":\"denied\"}" : body).utf8)
            let status = denied ? "\(self.denialStatus) Fixture denial" : "200 OK"
            if preferences || path.hasSuffix("/playback") {
                self.recordProgressSafety(preferences ? .playbackPreferences : .itemsPlayback,
                    status: denied ? self.denialStatus : 200, header: header)
            }
            let response = Data("HTTP/1.1 \(status)\r\nContent-Type: application/json\r\nContent-Length: \(payload.count)\r\nConnection: close\r\n\r\n".utf8) + payload
            if preferences && self.holdPreferences {
                self.heldPreferences.withLock { $0.append((connection, response)) }
                return
            }
            let delay = !delayed ? 0.0 : preferences ? 0.5 : id == "a" && count > 1 ? 1.0 : 0.2
            self.queue.asyncAfter(deadline: .now() + delay) {
                connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
            }
        }
    }
}
