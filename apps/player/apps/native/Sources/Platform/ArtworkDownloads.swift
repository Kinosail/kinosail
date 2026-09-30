import Foundation

/// Share encoded bytes across thumbnail sizes, while retaining independent cancellation.
actor ArtworkDownloads {
    private struct Key: Hashable, Sendable { let clientID: UUID; let path: String }
    private struct Pending {
        let id: UUID
        let task: Task<(Data, String), Error>
        var observers: Set<UUID>
    }
    private var pending: [Key: Pending] = [:]

    func resource(path: String, clientID: UUID,
                  fetch: @escaping @Sendable () async throws -> (Data, String)) async throws -> (Data, String) {
        try Task.checkCancellation()
        let key = Key(clientID: clientID, path: path)
        let observer = UUID()
        let request: Pending
        if var existing = pending[key] {
            existing.observers.insert(observer)
            pending[key] = existing
            request = existing
        } else {
            guard pending.count < 64 else { throw ClientError.unavailable }
            let id = UUID()
            let task = Task {
                defer { if pending[key]?.id == id { pending[key] = nil } }
                try Task.checkCancellation()
                return try await fetch()
            }
            request = Pending(id: id, task: task, observers: [observer])
            pending[key] = request
        }
        let requestID = request.id
        let result = try await withTaskCancellationHandler {
            try await request.task.value
        } onCancel: {
            Task { await self.stopWaiting(key: key, requestID: requestID, observerID: observer) }
        }
        try Task.checkCancellation()
        return result
    }

    func clear() {
        for request in pending.values { request.task.cancel() }
        pending = [:]
    }

    private func stopWaiting(key: Key, requestID: UUID, observerID: UUID) {
        guard var request = pending[key], request.id == requestID else { return }
        request.observers.remove(observerID)
        if request.observers.isEmpty {
            pending[key] = nil
            request.task.cancel()
        } else { pending[key] = request }
    }
}
