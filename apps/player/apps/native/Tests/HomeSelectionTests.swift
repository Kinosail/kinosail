import Testing
@testable import KinosailPlayer

struct HomeSelectionTests {
    @Test func keepsOnlyNewTitlesInRecentlyAdded() throws {
        let server = try ServerAddress("https://media.example")
        let watching = try item("watching", server: server)
        let continuation = try item("continuation", server: server)
        let new = try item("new", server: server)
        let selection = HomeSelection(continueWatching: [watching, continuation],
                                      recent: [watching, new, continuation])
        #expect(selection.featured?.id == watching.id)
        #expect(selection.continuation.map(\.id) == [continuation.id])
        #expect(selection.recent.map(\.id) == [new.id])
    }

    @Test func singleRecentTitleAppearsOnlyInTheFeature() throws {
        let server = try ServerAddress("https://media.example")
        let new = try item("new", server: server)
        let selection = HomeSelection(continueWatching: [], recent: [new])
        #expect(selection.featured?.id == new.id)
        #expect(selection.continuation.isEmpty)
        #expect(selection.recent.isEmpty)
    }

    @Test func continuationKeepsTheExistingFourRowLimit() throws {
        let server = try ServerAddress("https://media.example")
        let watching = try (0..<7).map { try item("item-\($0)", server: server) }
        let selection = HomeSelection(continueWatching: watching, recent: [])
        #expect(selection.featured?.id == "item-0")
        #expect(selection.continuation.map(\.id) == ["item-1", "item-2", "item-3", "item-4"])
    }

    @Test func emptyLibraryKeepsTheEmptyState() {
        let selection = HomeSelection(continueWatching: [], recent: [])
        #expect(selection.featured == nil)
        #expect(selection.continuation.isEmpty)
        #expect(selection.recent.isEmpty)
    }

    private func item(_ id: String, server: ServerAddress) throws -> MediaItem {
        try MediaItem(.object(["id": .string(id), "kind": .string("video"), "title": .string(id)]), server: server)
    }
}
