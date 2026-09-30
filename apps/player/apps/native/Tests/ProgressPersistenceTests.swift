import CryptoKit
import Foundation
import Testing
@testable import KinosailPlayer

struct ProgressPersistenceTests {
    @Test func defaultJournalUsesSupportedStorageAndRestoresPosition() async throws {
        let scope = SHA256.hash(data: Data(UUID().uuidString.utf8)).map { String(format: "%02x", $0) }.joined()
        #if os(tvOS)
        let location = FileManager.SearchPathDirectory.cachesDirectory
        #else
        let location = FileManager.SearchPathDirectory.applicationSupportDirectory
        #endif
        let file = FileManager.default.urls(for: location, in: .userDomainMask)[0]
            .appendingPathComponent("KinosailProgress").appendingPathComponent(scope + ".json")
        defer { try? FileManager.default.removeItem(at: file) }
        let store = try ProgressSyncStore(scope: scope)
        var progress = WatchProgress()
        progress.seconds = 42; progress.session = "playback"; progress.revision = 1
        try await store.record(itemID: "movie", progress: progress, expected: WatchProgress())
        #expect(FileManager.default.fileExists(atPath: file.path))
        let journal = try StrictJSON.decode(Data(contentsOf: file))
        let values = try journal.object(allowing: ["version", "entries"]).required("entries").array(max: 50)
        let entry = try #require(values.first)
        #expect(try PendingProgress(entry).progress == progress)
        let restored = try ProgressSyncStore(scope: scope)
        #expect(try await restored.pending().first?.progress == progress)
    }
}
