import Foundation
import Network
import Synchronization
import Testing
@testable import KinosailPlayer

final class SourceLoopbackFixture: @unchecked Sendable {
    struct Request: Sendable { let path: String; let query: [String: String] }
    struct Reply: @unchecked Sendable {
        var status = 200
        var body: [String: Any] = [:]
        var held = false
        var delayMilliseconds = 0
    }
    private struct State {
        var held: [(NWConnection, Reply)] = []
        var inFlight = 0
        var maximum = 0
        var paths: [String] = []
    }
    private let state = Mutex(State())
    private let listener: NWListener
    private let barrier: Int
    private let route: @Sendable (Request) -> Reply
    let address: URL
    var maximumHeld: Int { state.withLock { $0.maximum } }
    var paths: [String] { state.withLock { $0.paths } }

    init(barrier: Int, route: @escaping @Sendable (Request) -> Reply) async throws {
        self.barrier = barrier
        self.route = route
        let parameters = NWParameters.tcp
        parameters.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: .any)
        listener = try NWListener(using: parameters)
        let ready = Mutex(false)
        listener.stateUpdateHandler = { if case .ready = $0 { ready.withLock { $0 = true } } }
        listener.newConnectionHandler = { $0.cancel() }
        listener.start(queue: .global())
        for _ in 0..<250 where !ready.withLock({ $0 }) { try await Task.sleep(for: .milliseconds(10)) }
        try #require(ready.withLock { $0 })
        let port = try #require(listener.port).rawValue
        address = try #require(URL(string: "http://127.0.0.1:\(port)"))
        listener.newConnectionHandler = { [weak self] connection in
            connection.start(queue: .global())
            self?.receive(connection, data: Data())
        }
    }

    func account(_ kind: SourceKind) -> SourceAccount {
        SourceAccount(kind: kind, serverID: "fixture-server", userID: "fixtureuser", name: "Fixture",
                      userName: "Viewer", addresses: [address], token: "fixture-token")
    }

    func close() {
        listener.cancel()
        state.withLock { $0.held.forEach { $0.0.cancel() }; $0.held = [] }
    }

    private func receive(_ connection: NWConnection, data: Data) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 16384) { [weak self] chunk, _, complete, error in
            guard let self, let chunk, error == nil, data.count + chunk.count <= 16384 else { connection.cancel(); return }
            let next = data + chunk
            guard let boundary = next.range(of: Data("\r\n\r\n".utf8)),
                  let header = String(data: next[..<boundary.lowerBound], encoding: .utf8) else {
                if !complete { receive(connection, data: next) } else { connection.cancel() }; return
            }
            let parts = header.components(separatedBy: "\r\n")[0].split(separator: " ")
            guard parts.count == 3, let url = URLComponents(string: String(parts[1])) else { connection.cancel(); return }
            let query = Dictionary((url.queryItems ?? []).map { ($0.name, $0.value ?? "") }) { $1 }
            let reply = route(Request(path: url.path, query: query))
            guard reply.held else {
                state.withLock { $0.paths.append(url.path) }
                send(connection, reply); return
            }
            let release = state.withLock { value -> [(NWConnection, Reply)] in
                value.paths.append(url.path)
                value.held.append((connection, reply))
                value.inFlight += 1
                value.maximum = max(value.maximum, value.inFlight)
                guard value.held.count >= barrier else { return [] }
                defer { value.held = [] }
                return value.held
            }
            release.forEach { send($0.0, $0.1) }
            if release.isEmpty {
                DispatchQueue.global().asyncAfter(deadline: .now() + 1.5) { [weak self] in
                    guard let self else { return }
                    let late = state.withLock { value in
                        let found = value.held.filter { $0.0 === connection }
                        value.held.removeAll { $0.0 === connection }
                        return found
                    }
                    late.forEach { send($0.0, $0.1) }
                }
            }
        }
    }

    private func send(_ connection: NWConnection, _ reply: Reply) {
        DispatchQueue.global().asyncAfter(deadline: .now() + .milliseconds(reply.delayMilliseconds)) { [weak self] in
            let data = (try? JSONSerialization.data(withJSONObject: reply.body)) ?? Data("{}".utf8)
            let head = "HTTP/1.1 \(reply.status) Fixture\r\nContent-Type: application/json\r\nContent-Length: \(data.count)\r\nConnection: close\r\n\r\n"
            connection.send(content: Data(head.utf8) + data, completion: .contentProcessed { [weak self] _ in
                if reply.held { self?.state.withLock { $0.inFlight -= 1 } }
                connection.cancel()
            })
        }
    }
}
