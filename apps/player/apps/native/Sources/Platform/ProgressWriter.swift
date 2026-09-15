import Foundation

actor ProgressWriter {
    private let itemID: String
    private let client: ServerClient
    private let store: ProgressSyncStore
    private var baseline: WatchProgress
    private var progress = WatchProgress()

    init(itemID: String, expected: WatchProgress, client: ServerClient, store: ProgressSyncStore) throws {
        self.itemID = try Input.id(itemID)
        baseline = try expected.validated(required: false)
        self.client = client
        self.store = store
        progress.session = UUID().uuidString
    }

    /// Records locally before attempting HTTP. A failed connection cannot lose
    /// the update; the caller can distinguish pending sync from a storage failure.
    func update(seconds: Double, watched: Bool) async throws -> Bool {
        try Input.position(seconds)
        progress.revision += 1
        progress.seconds = seconds
        progress.watched = watched
        try await store.record(itemID: itemID, progress: progress, expected: baseline)
        do {
            let results = try await store.synchronize(client: client)
            if let current = results[itemID], !current.conflict { baseline = current.progress }
            return try await !store.pending().contains(where: { $0.itemID == itemID })
        } catch { return false }
    }
}
