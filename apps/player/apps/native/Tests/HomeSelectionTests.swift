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

    @Test func eachModeKeepsOnlyItsMedia() throws {
        let server = try ServerAddress("https://media.example")
        let movie = try item("movie", kind: "video", server: server)
        let album = try item("album", kind: "music", server: server)
        let book = try item("book", kind: "audiobook", server: server)
        let watch = HomeSelection(continueWatching: [album, movie], recent: [book, movie], mode: .watch)
        let listen = HomeSelection(continueWatching: [album, movie], recent: [book, movie], mode: .listen)
        #expect(watch.featured?.id == movie.id)
        #expect(watch.recent.isEmpty)
        #expect(listen.featured?.id == album.id)
        #expect(listen.recent.map(\.id) == [book.id])
    }

    private func item(_ id: String, kind: String = "video", server: ServerAddress) throws -> MediaItem {
        try MediaItem(.object(["id": .string(id), "kind": .string(kind), "title": .string(id)]), server: server)
    }
}
