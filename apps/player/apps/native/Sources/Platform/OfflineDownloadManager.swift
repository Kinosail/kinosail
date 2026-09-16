#if os(iOS)
import Foundation
import Observation

@MainActor @Observable
final class OfflineDownloadManager {
    private(set) var downloads: [OfflineDownload] = []
    private(set) var message: String?
    var busy = false
    var preferences = MediaPreferences()
    let engine = VerifiedDownloads.shared
    var catalog = OfflineCatalog()
    var storage: OfflineCatalogStore?
    var scope: String?
    var client: ServerClient?
    var generation = UUID()
    private var playback = false
    var operation = false
    private var completedItems: [OfflineCompletion] = []
    private var smartRetry = Date.distantPast
    private var processingSmart = false

    func restore(viewer: Viewer, server: ServerAddress, client: ServerClient) async {
        let attempt = UUID(); generation = attempt
        downloads = []; catalog = OfflineCatalog(); scope = nil; message = nil; storage = nil; self.client = nil; completedItems = []; smartRetry = .distantPast
        guard viewer.downloads else { await engine.lock(); return }
        do {
            let access = try await client.downloadAuthorization()
            guard generation == attempt else { return }
            guard access.server == server, access.profileID == viewer.id else { throw ClientError.invalidResponse }
            let storage = try OfflineCatalogStore(scope: access.scope, server: server)
            let saved = try await storage.load()
            guard generation == attempt else { return }
            try await engine.authorize(access, wifiOnly: saved.preferences.wifiOnly, quota: Int64(saved.preferences.downloadLimitGiB) * 1_073_741_824)
            guard generation == attempt else { return }
            self.storage = storage; self.scope = access.scope; self.client = client
            catalog = saved; preferences = saved.preferences
            completedItems = try await storage.completions()
            try check(attempt)
            for record in saved.records where record.deleting {
                try await engine.remove(access.scope, key: record.key)
                try check(attempt)
                var next = catalog; next.records.removeAll { $0.key == record.key }
                try await storage.save(next); try check(attempt); catalog = next
            }
            let snapshots = try await engine.snapshot(access.scope)
            for record in catalog.records where record.error.isEmpty && !snapshots.contains(where: { $0.key == record.key }) {
                try check(attempt)
                try await engine.enqueuePreparation(scope: access.scope, key: record.key,
                                                    uri: server.mediaURL("/api/v1/downloads/\(record.jobID)/file").absoluteString,
                                                    kind: record.item.kind == .music ? "audio" : record.item.kind.rawValue, wifiOnly: preferences.wifiOnly, quota: quota)
            }
            try check(attempt)
            await refresh()
        } catch { if generation == attempt { await engine.lock(); message = AppSession.message(error) } }
    }

    func pause(id: String) async throws {
        guard !operation else { throw ClientError.invalidInput("Wait for the current download operation to finish.") }
        operation = true; defer { operation = false }
        let scope = try activeScope(id: id)
        try await engine.pause(scope, key: id)
        await refresh()
    }

    func resume(id: String) async throws {
        guard !operation else { throw ClientError.invalidInput("Wait for the current download operation to finish.") }
        operation = true; defer { operation = false }
        let scope = try activeScope(id: id)
        guard let record = catalog.records.first(where: { $0.key == id && !$0.deleting }), let client else { throw ClientError.invalidResponse }
        let attempt = generation
        let snapshots = try await engine.snapshot(scope)
        try check(attempt)
        if snapshots.first(where: { $0.key == id })?.manifest == nil {
            let prepared = try await client.prepareDownload(itemID: record.item.id, quality: record.quality, tracks: record.tracks)
            try check(attempt)
            guard prepared.id == record.jobID else { throw ClientError.invalidInput("This title changed on the Server. Remove this download and add it again.") }
            guard prepared.state != .failed else { throw ClientError.invalidInput(prepared.error ?? "The Server could not prepare this title.") }
        }
        if snapshots.contains(where: { $0.key == id }) { try await engine.resume(scope, key: id, wifiOnly: preferences.wifiOnly, quota: quota) }
        else {
            try await engine.enqueuePreparation(scope: scope, key: id, uri: client.server.mediaURL("/api/v1/downloads/\(record.jobID)/file").absoluteString,
                                                kind: record.item.kind == .music ? "audio" : record.item.kind.rawValue, wifiOnly: preferences.wifiOnly, quota: quota)
        }
        try check(attempt)
        if let index = catalog.records.firstIndex(where: { $0.key == id }) { catalog.records[index].error = ""; try await storage?.save(catalog) }
        await refresh()
    }

    func remove(id: String) async throws {
        let scope = try activeScope(id: id)
        guard !operation, let storage else { throw ClientError.invalidInput("Wait for the current download operation to finish.") }
        operation = true
        defer { operation = false }
        let attempt = generation
        var intent = catalog
        guard let index = intent.records.firstIndex(where: { $0.key == id }) else { throw ClientError.invalidResponse }
        intent.records[index].deleting = true
        try await storage.save(intent)
        try check(attempt)
        catalog = intent
        try await engine.remove(scope, key: id)
        try check(attempt)
        var next = catalog; next.records.removeAll { $0.key == id }
        try await storage.save(next)
        try check(attempt)
        catalog = next
        await refresh()
    }

    func verifiedFile(id: String) async throws -> URL {
        let scope = try activeScope(id: id)
        let attempt = generation
        try await engine.check(scope, key: id)
        let deadline = Date().addingTimeInterval(120)
        while Date() < deadline {
            try check(attempt)
            let snapshot = try await engine.snapshot(scope).first { $0.key == id }
            guard let snapshot else { throw ClientError.invalidInput("The download is no longer available.") }
            if snapshot.status == "complete" { return try await engine.file(scope, key: id) }
            guard snapshot.status == "verifying" else { throw ClientError.invalidInput(snapshot.message.isEmpty ? "This download is not ready. Resume it to repair missing data." : snapshot.message) }
            try await Task.sleep(for: .milliseconds(200))
        }
        throw ClientError.invalidInput("Verification is still in progress. Try opening this download again shortly.")
    }

    func clear() async throws {
        for item in downloads { try await remove(id: item.id) }
    }

    func resetDeviceStorage() async throws {
        guard !operation else { throw ClientError.invalidInput("Wait for the current download operation to finish.") }
        operation = true; defer { operation = false }
        let attempt = generation
        try await engine.reset()
        try check(attempt)
        try OfflineCatalogStore.reset()
        catalog = OfflineCatalog(); downloads = []; message = nil
    }

    func lock() async {
        generation = UUID(); downloads = []; catalog = OfflineCatalog(); scope = nil; client = nil; storage = nil; completedItems = []
        await engine.lock()
    }

    func setPlaybackActive(_ active: Bool) {
        playback = active
        engine.setPlayback(active)
    }

    func updatePreferences(_ value: MediaPreferences) async throws {
        let valid = try MediaPreferences(value.json)
        guard let storage else { return }
        guard !operation else { throw ClientError.invalidInput("Wait for the current download operation to finish.") }
        operation = true; defer { operation = false }
        let attempt = generation
        var next = catalog; next.preferences = valid
        try await storage.save(next)
        try check(attempt)
        catalog = next; preferences = valid
        if let scope { try await engine.updatePolicy(scope: scope, wifiOnly: valid.wifiOnly, quota: quota); try check(attempt) }
    }

    func completed(_ item: MediaItem, nextID: String?, client: ServerClient) async {
        guard let scope, let storage else { return }
        let attempt = generation
        do {
            guard try await client.profileScope() == scope else { return }
            try check(attempt)
            let completion = try OfflineCompletion(item: item, nextID: nextID, quality: catalog.records.first(where: { $0.item.id == item.id })?.quality ?? .compatible)
            let pending = try await storage.recordCompletion(completion)
            try check(attempt)
            completedItems = pending
            await processCompleted()
        } catch { if generation == attempt { message = AppSession.message(error) } }
    }

    private func processCompleted() async {
        guard !operation, !processingSmart, Date() >= smartRetry, let completion = completedItems.first, let client, let storage else { return }
        processingSmart = true
        let attempt = generation
        defer { processingSmart = false }
        do {
            let quality = completion.quality
            var replacements: [String] = []
            if completion.item.kind == .video, !completion.item.showID.isEmpty, preferences.autoDownloadNext > 0 {
                var nextID = completion.nextID
                if nextID == nil { nextID = try await client.playback(itemID: completion.item.id).nextItemID }
                var seen = Set([completion.item.id])
                for _ in 0..<preferences.autoDownloadNext {
                    try check(attempt)
                    guard let id = nextID, seen.insert(id).inserted else { break }
                    let next = try await client.item(id: id)
                    try check(attempt)
                    guard next.kind == .video, next.showID == completion.item.showID else { throw ClientError.invalidResponse }
                    if !catalog.records.contains(where: { $0.item.id == id }) {
                        let selection: DownloadTrackSelection?
                        if quality == .original { selection = nil }
                        else { selection = try await DownloadTrackSelection(audio: client.downloadTracks(itemID: id).audio.map(\.index), subtitles: []) }
                        try await enqueue(item: next, quality: quality, tracks: selection, client: client)
                    }
                    replacements.append(id)
                    nextID = try await client.playback(itemID: id).nextItemID
                }
            }
            try check(attempt)
            if preferences.removeWatched {
                // Never delete the last playable episode while its replacement is still transferring.
                guard replacements.allSatisfy({ id in downloads.contains(where: { $0.item.id == id && $0.state == .ready }) }) else {
                    smartRetry = Date().addingTimeInterval(60); return
                }
                for id in replacements {
                    guard let replacement = downloads.first(where: { $0.item.id == id && $0.state == .ready }) else { throw ClientError.invalidResponse }
                    _ = try await verifiedFile(id: replacement.id)
                    try check(attempt)
                }
                for record in catalog.records where record.item.id == completion.item.id { try await remove(id: record.key); try check(attempt) }
            }
            let pending = try await storage.removeCompletion(completion.item.id)
            try check(attempt)
            completedItems = pending
            message = nil
        } catch is CancellationError {} catch {
            if generation == attempt { message = "Automatic downloads are waiting. \(AppSession.message(error))"; smartRetry = Date().addingTimeInterval(60) }
        }
    }

    func monitor() async {
        while !Task.isCancelled {
            await refresh()
            await processCompleted()
            do { try await Task.sleep(for: .seconds(2)) } catch { return }
        }
    }

    func refresh() async {
        guard let scope else { return }
        let attempt = generation
        do {
            let snapshots = try await engine.snapshot(scope)
            try check(attempt)
            downloads = catalog.records.map { record in
                let snapshot = snapshots.first { $0.key == record.key }
                let state: OfflineDownload.State
                if record.deleting { state = .failed }
                else if snapshot?.status == "complete" { state = .ready }
                else if playback, ["preparing", "queued", "waiting", "downloading"].contains(snapshot?.status ?? "queued") { state = .waiting }
                else { state = OfflineDownload.State(rawValue: snapshot?.status ?? (record.error.isEmpty ? "queued" : "failed")) ?? .failed }
                let explanation = record.deleting ? "Removal interrupted. Remove this download again." : playback && state == .waiting ? "Downloads resume after playback." : snapshot?.message ?? record.error
                return OfflineDownload(id: record.key, item: record.item, quality: record.quality, state: state, receivedBytes: snapshot?.bytes ?? 0,
                                       totalBytes: snapshot?.total ?? 0, message: explanation.isEmpty ? nil : explanation)
            }
            message = nil
        } catch is CancellationError {} catch { if generation == attempt { message = AppSession.message(error) } }
    }

    var quota: Int64 { Int64(preferences.downloadLimitGiB) * 1_073_741_824 }
    private func activeScope(id: String) throws -> String {
        _ = try Input.hex(id, count: 64)
        guard let scope, catalog.records.contains(where: { $0.key == id }) else { throw ClientError.invalidInput("The download is unavailable for this Viewer Profile.") }
        return scope
    }
    func check(_ attempt: UUID) throws { try Task.checkCancellation(); guard generation == attempt else { throw CancellationError() } }
}
#endif
