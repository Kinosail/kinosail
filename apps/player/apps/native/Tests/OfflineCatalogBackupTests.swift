#if os(iOS) && targetEnvironment(simulator)
import Foundation
import Testing
@testable import KinosailPlayer

// Runs alone before other native contracts in a fresh hosted app container.
// Never remove an existing root to manufacture the first-write precondition.
@Suite(.enabled(if: ProcessInfo.processInfo.environment["KINOSAIL_FRESH_BACKUP_TEST"] == "1"))
struct OfflineCatalogBackupTests {
    @Test func firstCompletionAndExistingHistoryStayExcludedFromBackup() async throws {
        let files = FileManager.default
        var root = OfflineCatalogStore.root
        try #require(!files.fileExists(atPath: root.path))
        defer { try? files.removeItem(at: root) }
        let server = try ServerAddress("https://backup-fixture.invalid")
        let scope = String(repeating: "b", count: 64)
        let store = try OfflineCatalogStore(scope: scope, server: server)
        #expect(try await store.load().records.isEmpty)
        #expect(try await store.completions().isEmpty)
        try #require(!files.fileExists(atPath: root.path))
        let item = try MediaItem(.object(["id": .string("backup-fixture"), "kind": .string("video"),
                                         "title": .string("Synthetic completion")]), server: server)
        let completion = try OfflineCompletion(item: item, nextID: "backup-next", quality: .compatible)
        let recorded = try await store.recordCompletion(completion)
        try #require(recorded.count == 1)
        #expect(recorded[0].item.id == item.id)
        #expect(try root.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup == true)
        let reopened = try OfflineCatalogStore(scope: scope, server: server)
        let restored = try await reopened.completions()
        try #require(restored.count == 1)
        #expect(restored[0].item.id == item.id)
        #expect(restored[0].nextID == "backup-next")
        #expect(restored[0].quality == .compatible)

        // Model an older directory whose journal exists without the flag.
        var attributes = URLResourceValues(); attributes.isExcludedFromBackup = false
        try root.setResourceValues(attributes)
        try #require(try root.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup == false)
        #expect(try await reopened.completions().count == 1)
        #expect(try root.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup == true)

        try root.setResourceValues(attributes)
        #expect(try await reopened.removeCompletion(item.id).isEmpty)
        #expect(try root.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup == true)
        #expect(try await reopened.completions().isEmpty)

        // The existing catalog writer proves that this filesystem supports the flag.
        try root.setResourceValues(attributes)
        try await reopened.save(OfflineCatalog())
        #expect(try root.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup == true)
        #expect(try await reopened.load().records.isEmpty)
        try files.removeItem(at: root)
        #expect(!files.fileExists(atPath: root.path))
        print("OFFLINE backup contract: first completion, legacy read, removal, catalog control, and cleanup checked")
    }
}
#endif
