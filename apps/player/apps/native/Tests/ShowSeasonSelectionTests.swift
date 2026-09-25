import Testing
@testable import KinosailPlayer

struct ShowSeasonSelectionTests {
    @Test func featureMatchesTheEpisodeItsActionWillPlay() throws {
        let server = try ServerAddress("https://media.example")
        let watched = try episode("watched", watched: true, server: server)
        let next = try episode("next", watched: false, server: server)
        #expect(ShowSeasonSelection.featuredEpisode(in: [watched, next])?.id == "next")
        #expect(ShowSeasonSelection.featuredEpisode(in: [watched])?.id == "watched")
        #expect(ShowSeasonSelection.featuredEpisode(in: []) == nil)
    }

    @Test func initiallyShowsTheFeaturedEpisodeSeason() {
        #expect(ShowSeasonSelection.resolve(nil, among: [1, 2, 3], defaultingTo: 3) == 3)
        #expect(ShowSeasonSelection.resolve(2, among: [1, 2, 3], defaultingTo: 3) == 2)
        #expect(ShowSeasonSelection.resolve(4, among: [1, 2, 3], defaultingTo: 3) == 3)
        #expect(ShowSeasonSelection.resolve(nil, among: [1, 2, 3], defaultingTo: 4) == 1)
    }

    private func episode(_ id: String, watched: Bool, server: ServerAddress) throws -> MediaItem {
        try MediaItem(.object(["id": .string(id), "kind": .string("video"), "title": .string(id),
                               "progress": .object(["watched": .bool(watched)])]), server: server)
    }
}
