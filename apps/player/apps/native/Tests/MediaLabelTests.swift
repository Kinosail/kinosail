import Testing
@testable import KinosailPlayer

struct MediaLabelTests {
    @Test func showLabelsOmitReleaseNamesIncludingCachedResponses() throws {
        let server = try ServerAddress("https://media.example")
        for kind in ["video", "show"] {
            for show in ["Age of Attraction", "Age of Attraction (2026) {imdb tt1234567}"] {
                let item = try MediaItem(.object([
                    "id": .string("sample"), "kind": .string(kind), "title": .string("Age of Attraction"),
                    "show": .string(show), "showId": .string("0123456789abcdef"),
                    "season": .number(1), "episode": .number(1),
                    "progress": .object(["seconds": .number(44)])
                ]), server: server)
                #expect(item.title == "Age of Attraction")
                #expect(item.subtitle.isEmpty)
                #expect(item.showID == "0123456789abcdef")
                #expect(item.progress.seconds == 44)
                #expect(item.playLabel == "Resume")
                let restored = try MediaItem(item.json, server: server)
                #expect(restored.subtitle.isEmpty)
                #expect(restored == item)
            }
        }
    }

    @Test func episodeTitlesAndMovieAndMusicMetadataRemainAvailable() throws {
        let server = try ServerAddress("https://media.example")
        let episode = try MediaItem(.object([
            "id": .string("episode"), "kind": .string("video"),
            "title": .string("S01E01 · Pilot"), "show": .string("Series"),
            "season": .number(1), "episode": .number(1)
        ]), server: server)
        #expect(episode.title == "S01E01 · Pilot")
        #expect(episode.subtitle.isEmpty)
        for (kind, title, artist, expected) in [
            ("video", "1917", "", "2019 · PG-13"),
            ("audio", "Track", "Artist", "Artist · 2019 · PG-13")
        ] {
            let item = try MediaItem(.object([
                "id": .string("sample"), "kind": .string(kind), "title": .string(title),
                "artist": .string(artist), "year": .string("2019"), "rating": .string("PG-13")
            ]), server: server)
            #expect(item.title == title)
            #expect(item.subtitle == expected)
        }
    }
}
