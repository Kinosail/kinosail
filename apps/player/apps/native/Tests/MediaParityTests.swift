import Foundation
import Testing
@testable import KinosailPlayer

struct MediaParityTests {
    @Test func serverDatesAcceptBoundedRFC3339Offsets() throws {
        let utc = try Input.date("2026-09-12T17:12:29.677181668Z")
        #expect(try Input.date("2026-09-12T11:12:29.677181668-06:00") == utc)
        #expect(try Input.date("2026-09-12T22:42:29.677181668+05:30") == utc)
        for suffix in ["+24:00", "+01:60", "+1:00", "+0100", "", "Zextra", "Z\n"] {
            #expect(throws: ClientError.self) { try Input.date("2026-09-12T17:12:29" + suffix) }
        }
    }

    @Test @MainActor func everyServerMediaKindHasItsOwnOpenPath() throws {
        let server = try ServerAddress("https://media.example")
        let cases: [(String, ScreenDestination, String)] = [
            ("video", .playback("sample"), "Play"),
            ("audio", .audio("sample"), "Play"),
            ("audiobook", .audio("sample"), "Play"),
            ("book", .reader("sample"), "Read"),
            ("photo", .photos("sample"), "View photo")
        ]
        for (kind, destination, label) in cases {
            let item = try MediaItem(.object(["id": .string("sample"), "kind": .string(kind), "title": .string("Sample")]), server: server)
            #expect(item.playingDestination == destination)
            #expect(item.playLabel == label)
        }
    }

    @Test func looseMusicPlaysWithoutRequestingAMissingAlbumQueue() async throws {
        let fixture = try HTTPFixture(body: "{}", status: 404)
        defer { fixture.remove() }
        let item = try await music(server: fixture.client.server)
        let queue = try await fixture.client.musicQueue(item: item)
        #expect(queue.map(\.id) == ["sample"])
        #expect(fixture.requests.isEmpty)
    }

    @Test func albumTracksKeepTheServerQueueOrder() async throws {
        let fixture = try HTTPFixture(body: """
        {"items":[{"id":"sample","kind":"audio","title":"First","album":"Album"},
                  {"id":"second","kind":"audio","title":"Second","album":"Album"}]}
        """)
        defer { fixture.remove() }
        let queue = try await fixture.client.musicQueue(item: music(server: fixture.client.server, album: "Album"))
        #expect(queue.map(\.id) == ["sample", "second"])
        #expect(fixture.requests.map { $0.url?.path } == ["/api/v1/audio/sample/queue"])
    }

    @Test(arguments: ["video", "audiobook", "book", "photo"])
    func rejectsNonMusicBeforeRequestingAQueue(_ kind: String) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let item = try await MediaItem(.object(["id": .string("sample"), "kind": .string(kind), "title": .string("Sample")]), server: fixture.client.server)
        await #expect(throws: ClientError.self) { try await fixture.client.musicQueue(item: item) }
        #expect(fixture.requests.isEmpty)
    }

    @Test(arguments: [
        "{\"items\":[]}",
        "{\"items\":[{\"id\":\"other\",\"kind\":\"audio\",\"title\":\"Other\"}]}",
        "{\"items\":[{\"id\":\"sample\",\"kind\":\"book\",\"title\":\"Book\"}]}",
        "{\"items\":[{\"id\":\"sample\",\"kind\":\"audio\",\"title\":\"A\"},{\"id\":\"sample\",\"kind\":\"audio\",\"title\":\"B\"}]}",
        "{\"items\":[{\"id\":\"sample\",\"kind\":\"audio\",\"title\":\"A\",\"stream\":\"https://other.example/media/sample\"}]}",
        "{\"items\":[],\"unknown\":true}"
    ])
    func rejectsInvalidRemoteQueues(_ body: String) async throws {
        let fixture = try HTTPFixture(body: body)
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.musicQueue(item: music(server: fixture.client.server, album: "Album")) }
        #expect(fixture.requests.count == 1)
        #expect(fixture.requests.allSatisfy { $0.httpMethod == "GET" })
    }

    private func music(server: ServerAddress, album: String = "") throws -> MediaItem {
        try MediaItem(.object(["id": .string("sample"), "kind": .string("audio"), "title": .string("Sample"), "album": .string(album)]), server: server)
    }
}
