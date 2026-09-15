import Foundation
import Network

/// State is confined to the gateway's serial queue, including Network callbacks.
final class MediaConnection: MediaTransferConsumer, @unchecked Sendable {
    private let connection: NWConnection
    private let queue: DispatchQueue
    private let transfer: MediaTransferSession
    private let port: UInt16
    private let request: (String) throws -> URLRequest
    private let rewrite: (Data, URL) throws -> Data
    private let finished: () -> Void
    private let failed: (Error) -> Void
    private var header = Data()
    private var manifest = Data()
    private var task: URLSessionDataTask?
    private var source: URL?
    private var isManifest = false
    private var isHead = false
    private var closed = false

    init(connection: NWConnection, queue: DispatchQueue, transfer: MediaTransferSession, port: UInt16,
         request: @escaping (String) throws -> URLRequest, rewrite: @escaping (Data, URL) throws -> Data, failed: @escaping (Error) -> Void = { _ in }, finished: @escaping () -> Void) {
        self.connection = connection; self.queue = queue; self.transfer = transfer; self.port = port
        self.request = request; self.rewrite = rewrite; self.failed = failed; self.finished = finished
    }

    func start() {
        connection.stateUpdateHandler = { [weak self] state in
            if case .failed = state { self?.close() }
            if case .cancelled = state { self?.close() }
        }
        connection.start(queue: queue)
        queue.asyncAfter(deadline: .now() + 10) { [weak self] in if self?.task == nil { self?.close() } }
        receiveHeader()
    }

    private func receiveHeader() {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 8192 - header.count) { [weak self] data, _, complete, error in
            guard let self, !self.closed else { return }
            if let data { self.header.append(data) }
            if let range = self.header.range(of: Data("\r\n\r\n".utf8)) {
                guard range.upperBound == self.header.count else { self.close(); return }
                do { try self.forward() } catch { self.close() }
            } else if complete || error != nil || self.header.count >= 8192 { self.close() }
            else { self.receiveHeader() }
        }
    }

    private func forward() throws {
        guard let raw = String(data: header, encoding: .utf8) else { throw ClientError.invalidResponse }
        let lines = raw.components(separatedBy: "\r\n")
        let first = (lines.first ?? "").split(separator: " ", omittingEmptySubsequences: false)
        guard first.count == 3, ["GET", "HEAD"].contains(String(first[0])), ["HTTP/1.1", "HTTP/1.0"].contains(String(first[2])) else { throw ClientError.invalidResponse }
        var names = Set<String>()
        var range: String?
        for line in lines.dropFirst() where !line.isEmpty {
            guard let colon = line.firstIndex(of: ":") else { throw ClientError.invalidResponse }
            let name = String(line[..<colon]).lowercased()
            let value = line[line.index(after: colon)...].trimmingCharacters(in: .whitespaces)
            guard name.range(of: "^[a-z0-9-]{1,64}$", options: .regularExpression) != nil, names.insert(name).inserted,
                  name != "transfer-encoding", name != "content-length", value.utf8.count <= 4096,
                  !value.unicodeScalars.contains(where: { CharacterSet.controlCharacters.contains($0) }) else { throw ClientError.invalidResponse }
            if name == "host", value != "127.0.0.1:\(port)" { throw ClientError.invalidResponse }
            if name == "range" { range = try MediaRange.validate(value) }
        }
        guard names.contains("host") else { throw ClientError.invalidResponse }
        var next = try request(String(first[1]))
        guard let url = next.url else { throw ClientError.invalidResponse }
        source = url
        isManifest = url.path.hasSuffix(".m3u8")
        isHead = first[0] == "HEAD"
        next.httpMethod = isManifest ? "GET" : String(first[0])
        next.timeoutInterval = 30
        next.setValue("identity", forHTTPHeaderField: "Accept-Encoding")
        if !isManifest { next.setValue(range, forHTTPHeaderField: "Range") }
        task = transfer.dataTask(with: next, consumer: self)
        guard let task else { throw ClientError.unavailable }
        task.resume()
    }

    func receive(_ response: URLResponse, completion: @escaping @Sendable (URLSession.ResponseDisposition) -> Void) {
        guard let http = response as? HTTPURLResponse, http.url == source else { failed(ClientError.invalidResponse); completion(.cancel); close(); return }
        guard [200, 206, 416].contains(http.statusCode) else { failed(ClientError.http(http.statusCode)); completion(.cancel); close(); return }
        guard !isManifest || (http.statusCode == 200 && http.expectedContentLength <= 2 * 1024 * 1024) else { failed(ClientError.invalidResponse); completion(.cancel); close(); return }
        if isManifest { completion(.allow); return }
        var output = "HTTP/1.1 \(http.statusCode) Media\r\nConnection: close\r\nCache-Control: no-store\r\n"
        for name in ["Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"] {
            if let value = http.value(forHTTPHeaderField: name) {
                guard value.utf8.count <= 512, !value.unicodeScalars.contains(where: { CharacterSet.controlCharacters.contains($0) }) else { failed(ClientError.invalidResponse); completion(.cancel); close(); return }
                output += "\(name): \(value)\r\n"
            }
        }
        connection.send(content: Data((output + "\r\n").utf8), completion: .contentProcessed { [weak self] error in
            if error != nil { self?.close(); completion(.cancel) } else { completion(.allow) }
        })
    }

    func receive(_ data: Data, task: URLSessionDataTask) {
        guard !closed else { return }
        if isManifest {
            guard data.count <= 2 * 1024 * 1024 - manifest.count else { failed(ClientError.invalidResponse); close(); return }
            manifest.append(data)
            return
        }
        task.suspend()
        connection.send(content: data, completion: .contentProcessed { [weak self] error in
            if error != nil { self?.close() } else if self?.closed == false { task.resume() }
        })
    }

    func complete(_ error: Error?) {
        guard !closed else { return }
        if let error { failed(error); close(); return }
        if isManifest {
            do {
                guard let source else { throw ClientError.invalidResponse }
                let body = try rewrite(manifest, source)
                var output = Data("HTTP/1.1 200 Media\r\nConnection: close\r\nCache-Control: no-store\r\nContent-Type: application/vnd.apple.mpegurl\r\nContent-Length: \(body.count)\r\n\r\n".utf8)
                if !isHead { output.append(body) }
                connection.send(content: output, completion: .contentProcessed { [weak self] error in if error != nil { self?.close() } })
            } catch { failed(error); close(); return }
        }
        connection.send(content: nil, isComplete: true, completion: .contentProcessed { [weak self] _ in self?.close() })
    }

    func close() {
        guard !closed else { return }
        closed = true
        connection.stateUpdateHandler = nil
        connection.cancel(); transfer.cancel(task); finished()
    }
}

enum MediaRange {
    static func validate(_ value: String) throws -> String {
        guard value.utf8.count <= 64, value.range(of: "^bytes=([0-9]{1,19}-[0-9]{0,19}|-[0-9]{1,19})$", options: .regularExpression) != nil else { throw ClientError.invalidResponse }
        let bounds = value.dropFirst(6).split(separator: "-", omittingEmptySubsequences: false)
        guard bounds.count == 2 else { throw ClientError.invalidResponse }
        if bounds[0].isEmpty {
            guard let suffix = UInt64(bounds[1]), suffix > 0 else { throw ClientError.invalidResponse }
        } else {
            guard let start = UInt64(bounds[0]), bounds[1].isEmpty || UInt64(bounds[1]).map({ $0 >= start }) == true else { throw ClientError.invalidResponse }
        }
        return value
    }
}
