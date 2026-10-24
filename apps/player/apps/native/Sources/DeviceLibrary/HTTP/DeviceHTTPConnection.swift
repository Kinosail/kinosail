import Foundation
import Network

final class DeviceHTTPConnection: MediaTransferConsumer, @unchecked Sendable {
    private static let headLimit = 16 * 1024
    private static let bodyLimit = 256 * 1024
    private static let manifestLimit = 2 * 1024 * 1024
    private let connection: NWConnection
    private let queue: DispatchQueue
    private let transfer: MediaTransferSession
    private let port: UInt16
    private let authorized: (String?) -> Bool
    private let handler: DeviceRequestHandler
    private let finished: () -> Void
    private var buffer = Data()
    private var head: String?
    private var expectedBody = 0
    private var isHead = false
    private var task: URLSessionDataTask?
    private var rewrite: DeviceManifestRewrite?
    private var manifest = Data()
    private var closed = false
    private var dispatched = false

    init(connection: NWConnection, queue: DispatchQueue, transfer: MediaTransferSession, port: UInt16,
         authorized: @escaping (String?) -> Bool, handler: DeviceRequestHandler, finished: @escaping () -> Void) {
        self.connection = connection; self.queue = queue; self.transfer = transfer; self.port = port
        self.authorized = authorized; self.handler = handler; self.finished = finished
    }

    func start() {
        connection.stateUpdateHandler = { [weak self] state in
            if case .failed = state { self?.close() }
            if case .cancelled = state { self?.close() }
        }
        connection.start(queue: queue)
        queue.asyncAfter(deadline: .now() + 15) { [weak self] in
            if self?.dispatched == false { self?.close() }
        }
        receive()
    }

    private func receive() {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 64 * 1024) { [weak self] data, _, complete, error in
            guard let self, !self.closed else { return }
            if let data { self.buffer.append(data) }
            do {
                if try self.parse() { return }
            } catch { self.reply(.failure(error)); return }
            if complete || error != nil { self.close() } else { self.receive() }
        }
    }

    private func parse() throws -> Bool {
        if head == nil {
            guard let range = buffer.range(of: Data("\r\n\r\n".utf8)) else {
                if buffer.count > Self.headLimit { throw DeviceFailure.invalid }
                return false
            }
            guard range.lowerBound <= Self.headLimit, let text = String(data: buffer[..<range.lowerBound], encoding: .utf8) else { throw DeviceFailure.invalid }
            head = text
            buffer = Data(buffer[range.upperBound...])
            let length = text.components(separatedBy: "\r\n").first { $0.lowercased().hasPrefix("content-length:") }
                .map { $0.dropFirst(15).trimmingCharacters(in: .whitespaces) } ?? "0"
            guard let count = Int(length), (0...Self.bodyLimit).contains(count) else { throw DeviceFailure.invalid }
            expectedBody = count
        }
        guard let head, buffer.count >= expectedBody else { return false }
        guard buffer.count == expectedBody else { throw DeviceFailure.invalid }
        let request = try DeviceRequest.parse(head: head, body: buffer, port: port)
        guard authorized(request.headers["authorization"]) else { throw DeviceFailure.unauthorized }
        isHead = request.method == "HEAD"
        dispatched = true
        let handler = handler
        Task { [weak self] in
            let response = await handler.handle(request)
            self?.queue.async { self?.reply(response, range: request.headers["range"]) }
        }
        return true
    }

    private func reply(_ response: DeviceResponse, range: String? = nil) {
        guard !closed else { return }
        switch response {
        case .json(let status, let value):
            let body = (try? JSONEncoder().encode(value)) ?? Data("{}".utf8)
            send(status: status, type: "application/json", body: body)
        case .empty(let status):
            send(status: status, type: nil, body: Data())
        case .data(let status, let body, let type):
            send(status: status, type: type, body: body)
        case .proxy(var request, let rewrite):
            self.rewrite = rewrite
            request.timeoutInterval = 30
            request.setValue("identity", forHTTPHeaderField: "Accept-Encoding")
            if rewrite == nil, let range, let valid = try? MediaRange.validate(range) { request.setValue(valid, forHTTPHeaderField: "Range") }
            if isHead && rewrite == nil { request.httpMethod = "HEAD" }
            guard let next = transfer.dataTask(with: request, consumer: self) else { reply(.failure(ClientError.unavailable)); return }
            task = next
            next.resume()
        }
    }

    private func send(status: Int, type: String?, body: Data, extra: String = "") {
        var output = "HTTP/1.1 \(status) Device\r\nConnection: close\r\nCache-Control: no-store\r\nContent-Length: \(body.count)\r\n"
        if let type { output += "Content-Type: \(type)\r\n" }
        var data = Data((output + extra + "\r\n").utf8)
        if !isHead { data.append(body) }
        connection.send(content: data, completion: .contentProcessed { _ in })
        connection.send(content: nil, isComplete: true, completion: .contentProcessed { [weak self] _ in self?.close() })
    }

    func receive(_ response: URLResponse, completion: @escaping @Sendable (URLSession.ResponseDisposition) -> Void) {
        guard let http = response as? HTTPURLResponse, [200, 206, 416].contains(http.statusCode) else {
            let code = (response as? HTTPURLResponse)?.statusCode ?? 502
            completion(.cancel); task = nil; reply(.failure(ClientError.http(code))); return
        }
        if rewrite != nil {
            guard http.expectedContentLength <= Self.manifestLimit else { completion(.cancel); reply(.failure(DeviceFailure.invalid)); return }
            completion(.allow); return
        }
        var output = "HTTP/1.1 \(http.statusCode) Media\r\nConnection: close\r\nCache-Control: no-store\r\n"
        for name in ["Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"] {
            if let value = http.value(forHTTPHeaderField: name), value.utf8.count <= 512,
               !value.unicodeScalars.contains(where: { CharacterSet.controlCharacters.contains($0) }) { output += "\(name): \(value)\r\n" }
        }
        connection.send(content: Data((output + "\r\n").utf8), completion: .contentProcessed { [weak self] error in
            if error != nil { self?.close(); completion(.cancel) } else { completion(.allow) }
        })
    }

    func receive(_ data: Data, task: URLSessionDataTask) {
        guard !closed else { return }
        if rewrite != nil {
            guard manifest.count + data.count <= Self.manifestLimit else { reply(.failure(DeviceFailure.invalid)); return }
            manifest.append(data); return
        }
        task.suspend()
        connection.send(content: data, completion: .contentProcessed { [weak self] error in
            if error != nil { self?.close() } else if self?.closed == false { task.resume() }
        })
    }

    func complete(_ error: Error?) {
        guard !closed, task != nil else { return }
        task = nil
        if let error { deviceLibraryLog.warning("operation=device-library-proxy outcome=failed code=\((error as? URLError)?.errorCode ?? 0)"); close(); return }
        if let rewrite {
            do {
                let body = try DeviceManifest.rewrite(manifest, rewrite: rewrite)
                send(status: 200, type: "application/vnd.apple.mpegurl", body: body)
            } catch { reply(.failure(error)) }
            return
        }
        connection.send(content: nil, isComplete: true, completion: .contentProcessed { [weak self] _ in self?.close() })
    }

    func close() {
        guard !closed else { return }
        closed = true
        connection.stateUpdateHandler = nil
        connection.cancel()
        transfer.cancel(task)
        finished()
    }
}
