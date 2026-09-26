import Testing
@testable import KinosailPlayer

struct HomeSelectionTests {
    @Test func recentShelvesKeepFeaturedAndContinuingTitles() throws {
        let server = try ServerAddress("https://media.example")
        let watching = try item("watching", server: server)
        let continuation = try item("continuation", server: server)
        let new = try item("new", server: server)
        let selection = HomeSelection(continueWatching: [watching, continuation],
                                      recent: [watching, new, continuation])
        #expect(selection.featured?.id == watching.id)
        #expect(selection.featuredIsContinuing)
        #expect(selection.continuation.map(\.id) == [continuation.id])
        #expect(selection.recent(for: .video).map(\.id) == [watching.id, new.id, continuation.id])
    }

    @Test func singleRecentTitleAlsoAppearsInItsShelf() throws {
        let server = try ServerAddress("https://media.example")
        let new = try item("new", server: server)
        let selection = HomeSelection(continueWatching: [], recent: [new])
        #expect(selection.featured?.id == new.id)
        #expect(!selection.featuredIsContinuing)
        #expect(selection.continuation.isEmpty)
        #expect(selection.recent(for: .video).map(\.id) == [new.id])
    }

    @Test func continuationKeepsAllTitlesAfterTheFeature() throws {
        let server = try ServerAddress("https://media.example")
        let watching = try (0..<7).map { try item("item-\($0)", server: server) }
        let selection = HomeSelection(continueWatching: watching, recent: [])
        #expect(selection.featured?.id == "item-0")
        #expect(selection.continuation.map(\.id) == ["item-1", "item-2", "item-3", "item-4", "item-5", "item-6"])
    }

    @Test func watchShelfShowsAtMostFifteenSavedTitlesAfterTheFeature() throws {
        let server = try ServerAddress("https://media.example")
        let watching = try (0..<18).map { try item("item-\($0)", server: server) }
        let selection = HomeSelection(continueWatching: watching, recent: [])
        #expect(selection.watchShelf.map(\.id) == (1...15).map { "item-\($0)" })
        #expect(selection.continuation.count == 17)
    }

    @Test func emptyLibraryKeepsTheEmptyState() {
        let selection = HomeSelection(continueWatching: [], recent: [])
        #expect(selection.featured == nil)
        #expect(!selection.featuredIsContinuing)
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
        #expect(watch.featuredIsContinuing)
        #expect(watch.recent(for: .video).map(\.id) == [movie.id])
        #expect(listen.featured?.id == album.id)
        #expect(listen.featuredIsContinuing)
        #expect(listen.recent(for: .music).isEmpty)
        #expect(listen.recent(for: .audiobook).map(\.id) == [book.id])
    }

    @Test func modeWithoutResumeUsesRecentFeature() throws {
        let server = try ServerAddress("https://media.example")
        let album = try item("album", kind: "music", server: server)
        let movie = try item("movie", kind: "video", server: server)
        let selection = HomeSelection(continueWatching: [album], recent: [movie], mode: .watch)
        #expect(selection.featured?.id == movie.id)
        #expect(!selection.featuredIsContinuing)
        #expect(selection.continuation.isEmpty)
    }

    @Test func separatesNewestItemsByWatchAndListenType() throws {
        let server = try ServerAddress("https://media.example")
        let movie = try item("movie", kind: "video", server: server)
        let show = try MediaItem(.object(["id": .string("episode"), "kind": .string("video"),
                                          "title": .string("Series"), "show": .string("Series"),
                                          "showId": .string("0123456789abcdef")]), server: server)
        let track = try item("track", kind: "music", server: server)
        let audiobook = try item("audiobook", kind: "audiobook", server: server)
        let recent = [movie, track, show, audiobook]
        let watch = HomeSelection(continueWatching: [], recent: recent, mode: .watch)
        let listen = HomeSelection(continueWatching: [], recent: recent, mode: .listen)
        #expect(watch.recent(for: .video).map(\.id) == ["movie"])
        #expect(watch.recent(for: .show).map(\.id) == ["episode"])
        #expect(listen.recent(for: .music).map(\.id) == ["track"])
        #expect(listen.recent(for: .audiobook).map(\.id) == ["audiobook"])
    }

    @Test func unwatchedShelvesKeepOnlyEligibleMoviesAndSeries() throws {
        let server = try ServerAddress("https://media.example")
        let unseenMovie = try item("unseen-movie", server: server)
        let seenMovie = try item("seen-movie", watched: true, server: server)
        let unseenSeries = try item("unseen-series", showID: "0123456789abcdef", server: server)
        let seenSeries = try item("seen-series", showID: "fedcba9876543210", watched: true, server: server)
        let selection = HomeSelection(continueWatching: [],
                                      recent: [unseenMovie, seenMovie, unseenSeries, seenSeries], mode: .watch)
        #expect(selection.unwatchedMovies.map(\.id) == ["unseen-movie"])
        #expect(selection.unwatchedShows.map(\.id) == ["unseen-series"])
    }

    @Test func movieGenreShelvesUseExactMovieMetadata() throws {
        let server = try ServerAddress("https://media.example")
        let drama = try item("drama", genres: "Drama · Mystery", server: server)
        let mystery = try item("mystery", genres: "Mystery", server: server)
        let series = try item("series", showID: "0123456789abcdef", genres: "Mystery", server: server)
        let selection = HomeSelection(continueWatching: [], recent: [drama, mystery, series], mode: .watch)
        #expect(selection.movieGenres.map(\.name) == ["Mystery", "Drama"])
        #expect(selection.movieGenres[0].items.map(\.id) == ["drama", "mystery"])
        #expect(selection.movieGenres[1].items.map(\.id) == ["drama"])
    }

    private func item(_ id: String, kind: String = "video", showID: String = "", watched: Bool = false,
                      genres: String = "", server: ServerAddress) throws -> MediaItem {
        try MediaItem(.object(["id": .string(id), "kind": .string(kind), "title": .string(id),
                               "showId": .string(showID), "genres": .string(genres),
                               "progress": .object(["watched": .bool(watched)])]), server: server)
    }
}
