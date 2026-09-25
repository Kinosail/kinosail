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
}
