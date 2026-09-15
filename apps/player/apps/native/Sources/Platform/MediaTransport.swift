import Foundation
import Network

actor MediaTransport {
    private var gateway: MediaGateway?

    func open(url: URL, itemID: String, client: ServerClient) async throws -> URL {
        let id = try Input.id(itemID)
        let request = try await client.authorizedRequest(url.absoluteString)
        guard url.path == "/media/\(id)" || url.path.hasPrefix("/hls/\(id)/") else { throw ClientError.invalidResponse }
        gateway?.close()
        let next = try MediaGateway(request: request, server: client.server, itemID: id)
        gateway = next
        do { return try await next.start() }
        catch { next.close(); throw error }
    }

    func failure() async -> Error? { await gateway?.failure() }

    func close() { gateway?.close(); gateway = nil }
}

/// All mutable state and delegate callbacks belong to this serial queue.
/// Only start and close cross that boundary; neither exposes credentials.
private final class MediaGateway: @unchecked Sendable {
    private let queue = DispatchQueue(label: "com.kinosail.player.media")
    private let listener: NWListener
    private let transfer: MediaTransferSession
    private let request: URLRequest
    private let server: ServerAddress
    private let itemID: String
    private let capability = "/" + UUID().uuidString + UUID().uuidString + "/"
    private var resources: [String: URL] = [:]
    private var paths: [URL: String] = [:]
    private var connections: [UUID: MediaConnection] = [:]
    private var started = false
    private var closed = false
    private var lastFailure: Error?

    init(request: URLRequest, server: ServerAddress, itemID: String) throws {
        self.request = request; self.server = server; self.itemID = itemID
        let parameters = NWParameters.tcp
        parameters.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: .any)
        listener = try NWListener(using: parameters)
        transfer = MediaTransferSession(queue: queue)
    }

    func start() async throws -> URL {
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { continuation in
                queue.async { [self] in
                    guard !closed else { continuation.resume(throwing: CancellationError()); return }
                    listener.stateUpdateHandler = { [self] state in
                        guard !started else { return }
                        switch state {
                        case .ready:
                            started = true
                            do {
                                guard let url = request.url else { throw ClientError.invalidResponse }
                                continuation.resume(returning: try localURL(for: url))
                            } catch { continuation.resume(throwing: error); close() }
                        case .failed, .cancelled:
                            started = true
                            continuation.resume(throwing: ClientError.unavailable)
                            listener.stateUpdateHandler = nil
                        default: break
                        }
                    }
                    listener.newConnectionHandler = { [weak self] connection in self?.accept(connection) }
                    listener.start(queue: queue)
                    queue.asyncAfter(deadline: .now() + 5) { [weak self] in
                        if self?.started == false { self?.listener.cancel() }
                    }
                }
            }
        } onCancel: { close() }
    }

    private func accept(_ connection: NWConnection) {
        guard !closed, connections.count < 4, let port = listener.port else { connection.cancel(); return }
        let id = UUID()
        let stream = MediaConnection(connection: connection, queue: queue, transfer: transfer, port: port.rawValue,
                                     request: { [weak self] path in
            guard let self, !self.closed, let url = self.resources[path] else { throw ClientError.invalidResponse }
            var request = self.request
            request.url = url
            return request
        }, rewrite: { [weak self] data, source in
            guard let self, !self.closed else { throw CancellationError() }
            let previousResources = self.resources, previousPaths = self.paths
            do { return try HLSManifest.rewrite(data, source: source, server: self.server, itemID: self.itemID) { try self.localURL(for: $0) } }
            catch { self.resources = previousResources; self.paths = previousPaths; throw error }
        }, failed: { [weak self] error in self?.lastFailure = error }, finished: { [weak self] in self?.connections.removeValue(forKey: id) })
        connections[id] = stream
        stream.start()
    }

    private func localURL(for remote: URL) throws -> URL {
        guard let port = listener.port else { throw ClientError.unavailable }
        let path: String
        if let existing = paths[remote] { path = existing }
        else {
            guard resources.count < 32_768 else { throw ClientError.invalidResponse }
            path = capability + String(resources.count) + (remote.path.hasSuffix(".m3u8") ? ".m3u8" : ".media")
            resources[path] = remote
            paths[remote] = path
        }
        guard let url = URL(string: "http://127.0.0.1:\(port.rawValue)\(path)") else { throw ClientError.invalidResponse }
        return url
    }

    func failure() async -> Error? {
        await withCheckedContinuation { continuation in queue.async { [self] in continuation.resume(returning: lastFailure) } }
    }

    func close() {
        queue.async { [self] in
            guard !closed else { return }
            closed = true
            listener.cancel()
            listener.newConnectionHandler = nil
            if started { listener.stateUpdateHandler = nil }
            let active = Array(connections.values)
            connections.removeAll()
            active.forEach { $0.close() }
            resources.removeAll(); paths.removeAll()
            transfer.close()
        }
    }
}

enum HLSManifest {
    static func rewrite(_ data: Data, source: URL, server: ServerAddress, itemID: String,
                        local: (URL) throws -> URL) throws -> Data {
        var count = 0
        _ = try transform(data, source: source, server: server, itemID: itemID) { url in
            count += 1
            guard count <= 32_768 else { throw ClientError.invalidResponse }
            return url
        }
        return try transform(data, source: source, server: server, itemID: itemID, local: local)
    }

    private static func transform(_ data: Data, source: URL, server: ServerAddress, itemID: String,
                        local: (URL) throws -> URL) throws -> Data {
        let id = try Input.id(itemID)
        guard try server.mediaURL(source.absoluteString).path.hasPrefix("/hls/\(id)/") else { throw ClientError.invalidResponse }
        guard data.count <= 2 * 1024 * 1024, let raw = String(data: data, encoding: .utf8), raw.hasPrefix("#EXTM3U\n") || raw.hasPrefix("#EXTM3U\r\n") else { throw ClientError.invalidResponse }
        let lines = raw.components(separatedBy: .newlines)
        guard lines.count <= 32_768 else { throw ClientError.invalidResponse }
        func mapped(_ reference: String) throws -> String {
            guard !reference.isEmpty, reference.utf8.count <= 16_384, !reference.contains("{$"),
                  let resolved = URL(string: reference, relativeTo: source)?.absoluteURL else { throw ClientError.invalidResponse }
            let url = try server.mediaURL(resolved.absoluteString)
            guard url.path.hasPrefix("/hls/\(id)/") else { throw ClientError.invalidResponse }
            return try local(url).absoluteString
        }
        let uri = try NSRegularExpression(pattern: #"(?:^|[,\-:])URI="([^"]*)""#)
        var output: [String] = []
        for rawLine in lines {
            guard rawLine.utf8.count <= 32_768, !rawLine.contains("#EXT-X-DEFINE"), !rawLine.contains("#EXT-X-CONTENT-STEERING") else { throw ClientError.invalidResponse }
            var line = rawLine.trimmingCharacters(in: .whitespaces)
            if !line.isEmpty, !line.hasPrefix("#") { line = try mapped(line) }
            else if line.contains("URI=") {
                let matches = uri.matches(in: line, range: NSRange(line.startIndex..., in: line))
                guard !matches.isEmpty, matches.count <= 16, matches.count == line.components(separatedBy: "URI=").count - 1 else { throw ClientError.invalidResponse }
                for match in matches.reversed() {
                    guard let range = Range(match.range(at: 1), in: line) else { throw ClientError.invalidResponse }
                    line.replaceSubrange(range, with: try mapped(String(line[range])))
                }
            }
            output.append(line)
        }
        let rewritten = Data(output.joined(separator: "\n").utf8)
        guard rewritten.count <= 8 * 1024 * 1024 else { throw ClientError.invalidResponse }
        return rewritten
    }
}
