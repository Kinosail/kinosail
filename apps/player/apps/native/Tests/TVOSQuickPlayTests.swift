import Testing
@testable import KinosailPlayer

struct TVOSQuickPlayTests {
    @Test func remotePlayOpensPlayableTitlesDirectly() throws {
        #expect(try TVOSQuickPlay.destination(for: item("video")) == .playback("video"))
        #expect(try TVOSQuickPlay.destination(for: item("music")) == .audio("music"))
        #expect(try TVOSQuickPlay.destination(for: item("audiobook")) == .audio("audiobook"))
    }

    @Test func remotePlayLeavesNonPlayableCardsForSelection() throws {
        for kind in ["show", "photo", "book"] {
            #expect(try TVOSQuickPlay.destination(for: item(kind)) == nil)
        }
    }

    private func item(_ kind: String) throws -> MediaItem {
        try MediaItem(.object(["id": .string(kind), "kind": .string(kind), "title": .string(kind)]),
                      server: ServerAddress("https://media.example"))
    }
}
