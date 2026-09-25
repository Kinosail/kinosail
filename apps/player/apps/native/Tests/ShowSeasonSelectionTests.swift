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

    @Test func groupsEverySeasonInOrderAndKeepsEpisodeAndProgressOrder() throws {
        let server = try ServerAddress("https://media.example")
        #expect(ShowSeasonSelection.groups([]).isEmpty)
        func item(_ id: String, season: Int, episode: Int, watched: Bool = false) throws -> MediaItem {
            try MediaItem(.object(["id": .string(id), "kind": .string("video"),
                                   "title": .string("S02E\(String(format: "%02d", episode)) · \(id)"),
                                   "season": .number(Double(season)), "episode": .number(Double(episode)),
                                   "progress": .object(["watched": .bool(watched)])]), server: server)
        }
        let groups = ShowSeasonSelection.groups([try item("second", season: 2, episode: 2),
                                                  try item("special", season: 0, episode: 1),
                                                  try item("first", season: 2, episode: 1, watched: true)])
        #expect(groups.map(\.number) == [0, 2])
        #expect(groups.map(\.title) == ["Specials", "Season 2"])
        #expect(groups[1].episodes.map(\.id) == ["second", "first"])
        #expect(groups[1].watchedCount == 1)
        #expect(ShowSeasonSelection.episodeTitle(groups[1].episodes[0]) == "second")
        #expect(ShowSeasonSelection.episodeTitle(groups[0].episodes[0]) == groups[0].episodes[0].title)
    }

    private func episode(_ id: String, watched: Bool, server: ServerAddress) throws -> MediaItem {
        try MediaItem(.object(["id": .string(id), "kind": .string("video"), "title": .string(id),
                               "progress": .object(["watched": .bool(watched)])]), server: server)
    }
}
