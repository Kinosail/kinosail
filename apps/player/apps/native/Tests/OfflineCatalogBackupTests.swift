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
        #expect(try excludedFromBackup(root) == true)
        let reopened = try OfflineCatalogStore(scope: scope, server: server)
        let restored = try await reopened.completions()
        try #require(restored.count == 1)
        #expect(restored[0].item.id == item.id)
        #expect(restored[0].nextID == "backup-next")
        #expect(restored[0].quality == .compatible)

        // Model an older directory whose journal exists without the flag.
        var attributes = URLResourceValues(); attributes.isExcludedFromBackup = false
        try root.setResourceValues(attributes)
        try #require(try excludedFromBackup(root) == false)
        let migrated = try await reopened.completions()
        try #require(migrated.count == 1)
        #expect(migrated[0].item.id == item.id)
        #expect(migrated[0].nextID == "backup-next")
        #expect(migrated[0].quality == .compatible)
        #expect(try excludedFromBackup(root) == true)

        try root.setResourceValues(attributes)
        #expect(try await reopened.removeCompletion(item.id).isEmpty)
        #expect(try excludedFromBackup(root) == true)
        #expect(try await reopened.completions().isEmpty)

        // The existing catalog writer proves that this filesystem supports the flag.
        try root.setResourceValues(attributes)
        try await reopened.save(OfflineCatalog())
        #expect(try excludedFromBackup(root) == true)
        #expect(try await reopened.load().records.isEmpty)
        try files.removeItem(at: root)
        #expect(!files.fileExists(atPath: root.path))
        let blocker = Data("synthetic blocked backup directory".utf8)
        try blocker.write(to: root)
        await #expect(throws: (any Error).self) { try await reopened.recordCompletion(completion) }
        #expect(try Data(contentsOf: root) == blocker)
        try files.removeItem(at: root)
        #expect(!files.fileExists(atPath: root.path))
        print("OFFLINE backup contract: first completion, legacy read, removal, catalog control, blocked write, and cleanup checked")
    }

    private func excludedFromBackup(_ url: URL) throws -> Bool? {
        var observed = url
        observed.removeCachedResourceValue(forKey: .isExcludedFromBackupKey)
        return try observed.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup
    }
}
#endif
