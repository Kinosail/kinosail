import Testing
@testable import KinosailPlayer

struct MediaDestinationTests {
    @Test func showsLibraryOpensSeasonPageWithoutChangingEpisodeDestinations() throws {
        let server = try ServerAddress("https://media.example")
        let episode = try MediaItem(.object([
            "id": .string("episode"), "kind": .string("video"), "title": .string("Example Show"),
            "showId": .string("0123456789abcdef")
        ]), server: server)
        let movie = try MediaItem(.object([
            "id": .string("movie"), "kind": .string("video"), "title": .string("Arrival")
        ]), server: server)

        #expect(episode.destination(inShows: true) == .show("0123456789abcdef"))
        #expect(episode.destination(inShows: false) == .detail("episode"))
        #expect(movie.destination(inShows: true) == .detail("movie"))
    }

    @Test func photoCardOpensTheViewerDirectlyOnTV() throws {
        let photo = try MediaItem(.object([
            "id": .string("photo"), "kind": .string("photo"), "title": .string("Coast")
        ]), server: ServerAddress("https://media.example"))

        #if os(tvOS)
        #expect(photo.destination(inShows: false) == .photos("photo"))
        #else
        #expect(photo.destination(inShows: false) == .detail("photo"))
        #endif
    }
}
