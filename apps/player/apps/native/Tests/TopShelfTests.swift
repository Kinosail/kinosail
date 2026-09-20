#if os(tvOS)
import Foundation
import Testing
@testable import KinosailPlayer

struct TopShelfTests {
    private let scope = String(repeating: "a", count: 64)
    private var item: ShelfSnapshot.Item {
        .init(id: "movie", title: "Sample movie", section: "Continue watching", image: Data([0xff, 0xd8, 0xff, 0xd9]))
    }

    @Test func preservesTopShelfOptOutDuringMigration() {
        let suite = "TopShelfTests.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        defaults.set(false, forKey: TopShelfPreferences.enabledKey)

        TopShelfPreferences.migrateToDefaultOn(in: defaults)
        #expect(!defaults.bool(forKey: TopShelfPreferences.enabledKey))

        defaults.set(false, forKey: TopShelfPreferences.enabledKey)
        TopShelfPreferences.migrateToDefaultOn(in: defaults)
        #expect(!defaults.bool(forKey: TopShelfPreferences.enabledKey))
    }

    @Test func enablesTopShelfWhenNoPreferenceExists() {
        let suite = "TopShelfTests.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        TopShelfPreferences.migrateToDefaultOn(in: defaults)
        #expect(defaults.bool(forKey: TopShelfPreferences.enabledKey))
    }

    @Test func roundTripAndCredentialFreeLinks() throws {
        let snapshot = ShelfSnapshot(scope: scope, saved: Date(), items: [item])
        let restored = try ShelfSnapshot.decode(JSONEncoder().encode(snapshot))
        #expect(restored.items.count == 1)
        for play in [true, false] {
            let link = try MediaLink(url: #require(restored.url(for: item, play: play)))
            #expect(link.scope == scope)
            #expect(link.value == item.id)
            #expect(link.action == (play ? .play : .detail))
        }
    }

    @Test(arguments: [-86_401.0, 120.0])
    func rejectsExpiredAndFutureSnapshots(offset: Double) throws {
        #expect(throws: (any Error).self) {
            try ShelfSnapshot(scope: scope, saved: Date().addingTimeInterval(offset), items: [item]).validated()
        }
    }

    @Test func rejectsConflictingAndOversizedCollections() {
        for items in [[item, item], Array(repeating: item, count: 19)] {
            #expect(throws: (any Error).self) { try ShelfSnapshot(scope: scope, saved: Date(), items: items).validated() }
        }
    }

    @Test func rejectsInvalidItemsBeforeWriting() {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        let invalid: [ShelfSnapshot.Item] = [
            .init(id: "../escape", title: item.title, section: item.section, image: item.image),
            .init(id: item.id, title: " ", section: item.section, image: item.image),
            .init(id: item.id, title: "a\nb", section: item.section, image: item.image),
            .init(id: item.id, title: String(repeating: "x", count: 2049), section: item.section, image: item.image),
            .init(id: item.id, title: item.title, section: "unknown", image: item.image),
            .init(id: item.id, title: item.title, section: item.section, image: Data()),
            .init(id: item.id, title: item.title, section: item.section, image: Data(repeating: 0xff, count: ShelfSnapshot.maximumImage + 1))
        ]
        for entry in invalid {
            #expect(throws: (any Error).self) { try ShelfSnapshot(scope: scope, saved: Date(), items: [entry]).write(to: directory) }
            #expect(!FileManager.default.fileExists(atPath: directory.path))
        }
    }

    @Test(arguments: ["", String(repeating: "a", count: 63), String(repeating: "A", count: 64)])
    func rejectsInvalidScopeWithoutFiles(value: String) {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        #expect(throws: (any Error).self) { try ShelfSnapshot(scope: value, saved: Date(), items: [item]).write(to: directory) }
        #expect(!FileManager.default.fileExists(atPath: directory.path))
    }

    @Test func rejectsUnknownDuplicateMissingAndMalformedFields() throws {
        let snapshot = ShelfSnapshot(scope: scope, saved: Date(), items: [item])
        let data = try JSONEncoder().encode(snapshot)
        let json = try #require(String(data: data, encoding: .utf8))
        for invalid in ["{}", "{", "{\"unexpected\":true," + json.dropFirst(),
                        "{\"scope\":\"" + scope + "\"," + json.dropFirst(),
                        json.replacingOccurrences(of: "\"title\":", with: "\"unexpected\":"),
                        String(repeating: " ", count: ShelfSnapshot.maximum + 1)] {
            #expect(throws: (any Error).self) { try ShelfSnapshot.decode(Data(invalid.utf8)) }
        }
    }
}
#endif
