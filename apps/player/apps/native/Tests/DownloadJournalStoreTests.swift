#if os(iOS)
import Foundation
import Testing
@testable import KinosailPlayer

struct DownloadJournalStoreTests {
    private let scope = String(repeating: "a", count: 64)
    private let key = String(repeating: "b", count: 64)

    private func plan(scope: String? = nil, key: String? = nil) -> DownloadPreparation {
        DownloadPreparation(scope: scope ?? self.scope, key: key ?? self.key,
                            uri: "https://media.example/api/v1/downloads/aaaaaaaaaaaaaaaa/file",
                            kind: "video", wifiOnly: true, quota: 0, status: "preparing", error: "")
    }

    @Test func savesAndRestoresProtectedPlans() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let store = DownloadJournalStore(directory: root)
        let input = plan()
        try store.save(input)
        let restored = try store.load()
        #expect(restored.jobs.isEmpty)
        #expect(restored.plans[input.id]?.uri == input.uri)
        #expect(try root.appendingPathComponent(scope).resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup == true)
        try store.removePlan(input)
        #expect(try store.load().plans.isEmpty)
    }

    @Test func rejectsInvalidPathsWithoutCreatingFiles() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let store = DownloadJournalStore(directory: root)
        for invalid in ["", "../outside", String(repeating: "a", count: 65), String(repeating: "g", count: 64)] {
            #expect(throws: ClientError.self) { try store.save(plan(scope: invalid)) }
            #expect(throws: ClientError.self) { try store.save(plan(key: invalid)) }
            #expect(throws: ClientError.self) { try store.remove(scope: invalid, key: key) }
            #expect(throws: ClientError.self) { try store.remove(scope: scope, key: invalid) }
            #expect(!FileManager.default.fileExists(atPath: root.path))
        }
    }

    @Test func isolatesCorruptAndMisplacedJournals() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let store = DownloadJournalStore(directory: root)
        let input = plan()
        try store.save(input)
        let folder = root.appendingPathComponent(scope)
        let corrupt = folder.appendingPathComponent(String(repeating: "c", count: 64) + ".planning")
        try Data("{broken".utf8).write(to: corrupt)
        let misplaced = folder.appendingPathComponent(String(repeating: "d", count: 64) + ".planning")
        try JSONEncoder().encode(input).write(to: misplaced)
        let restored = try store.load()
        #expect(restored.plans.count == 1)
        #expect(restored.plans[input.id] != nil)
        for file in [corrupt, misplaced] {
            #expect(!FileManager.default.fileExists(atPath: file.path))
            #expect(FileManager.default.fileExists(atPath: file.appendingPathExtension("invalid").path))
        }
    }

    @Test func finishesInterruptedDeletionBeforeRestoration() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let store = DownloadJournalStore(directory: root)
        try store.save(plan())
        let folder = root.appendingPathComponent(scope)
        for suffix in [".deleting", ".media", ".media.part", ".transfer.invalid"] {
            try Data().write(to: folder.appendingPathComponent(key + suffix))
        }
        #expect(try store.load().plans.isEmpty)
        #expect(try FileManager.default.contentsOfDirectory(atPath: folder.path).isEmpty)
    }

    @Test func rejectsSymlinkedScopeWithoutWritingToTarget() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let outside = root.appendingPathComponent("outside")
        try FileManager.default.createDirectory(at: outside, withIntermediateDirectories: true)
        try FileManager.default.createSymbolicLink(at: root.appendingPathComponent(scope), withDestinationURL: outside)
        let store = DownloadJournalStore(directory: root)
        #expect(throws: ClientError.self) { try store.save(plan()) }
        #expect(throws: ClientError.self) { try store.remove(scope: scope, key: key) }
        #expect(try store.load().plans.isEmpty)
        #expect(try FileManager.default.contentsOfDirectory(atPath: outside.path).isEmpty)
    }
}
#endif
