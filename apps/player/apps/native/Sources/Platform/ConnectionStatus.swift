import Foundation
import Network
import Observation

/// Network availability is a hint; only an actual Server response proves reachability.
@MainActor @Observable
final class ConnectionStatus {
    private(set) var networkAvailable: Bool?
    private(set) var server = ServerReachability.unknown
    private(set) var checking = false
    private var identity: UUID?
    private var checkID = UUID()
    private var observation = UUID()

    var unavailable: Bool { networkAvailable == false || server == .unreachable }
    var title: String { networkAvailable == false ? "You’re offline" : "Can’t reach your Server" }

    func monitor(_ client: ServerClient) async {
        let attempt = UUID()
        observation = attempt
        if identity != client.identity {
            identity = client.identity
            server = .unknown
            networkAvailable = nil
            checking = false
            checkID = UUID()
        }
        let monitor = NWPathMonitor()
        let (paths, continuation) = AsyncStream<Bool>.makeStream(bufferingPolicy: .bufferingNewest(1))
        monitor.pathUpdateHandler = { continuation.yield($0.status != .unsatisfied) }
        monitor.start(queue: DispatchQueue(label: "com.kinosail.connection"))
        defer { monitor.cancel(); continuation.finish() }
        await withTaskGroup(of: Void.self) { group in
            group.addTask { [weak self] in
                for await available in paths {
                    guard !Task.isCancelled else { return }
                    await self?.updateNetwork(available, client: client, attempt: attempt)
                }
            }
            group.addTask { [weak self] in
                for await state in await client.connectionUpdates() {
                    guard !Task.isCancelled else { return }
                    await self?.updateServer(state, client: client, attempt: attempt)
                }
            }
            await group.waitForAll()
        }
    }

    func check(_ client: ServerClient, force: Bool = false) async {
        guard !checking, identity == client.identity, force || networkAvailable != false else { return }
        checking = true
        let attempt = UUID()
        checkID = attempt
        defer { if checkID == attempt { checking = false } }
        // Never use a cache hit as evidence of a restored connection.
        _ = try? await client.viewer()
    }

    private func updateNetwork(_ available: Bool, client: ServerClient, attempt: UUID) {
        guard identity == client.identity, observation == attempt else { return }
        if networkAvailable == false && available { server = .unreachable }
        networkAvailable = available
    }

    private func updateServer(_ state: ServerReachability, client: ServerClient, attempt: UUID) {
        guard identity == client.identity, observation == attempt else { return }
        server = state
    }
}
