import Foundation

enum ServerReachability: Sendable { case unknown, reachable, unreachable }

extension ServerClient {
    func connectionUpdates() -> AsyncStream<ServerReachability> {
        guard !Task.isCancelled else { return AsyncStream { $0.finish() } }
        connectionObserver?.continuation.finish()
        let id = UUID()
        let (stream, continuation) = AsyncStream<ServerReachability>.makeStream(bufferingPolicy: .bufferingNewest(1))
        connectionObserver = (id, continuation)
        continuation.yield(reachability)
        continuation.onTermination = { [weak self] _ in
            Task { await self?.removeConnectionObserver(id) }
        }
        return stream
    }

    func recordConnection(_ state: ServerReachability, sequence: UInt64) {
        guard sequence >= connectionSequence else { return }
        connectionSequence = sequence
        reachability = state
        connectionObserver?.continuation.yield(state)
    }

    private func removeConnectionObserver(_ id: UUID) {
        if connectionObserver?.id == id { connectionObserver = nil }
    }
}
