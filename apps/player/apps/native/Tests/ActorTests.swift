import Foundation
import Testing
@testable import KinosailPlayer

struct ActorTests {
    private let showID = "0123456789abcdef"
    private var person: JSONValue {
        .object(["name": .string("Amy Adams"), "role": .string("Louise Banks"), "image": .string("/person/movie/0")])
    }

    @Test func retainsCastAcrossMediaAndSavedMetadataRoundTrips() throws {
        let server = try ServerAddress("https://media.example")
        let item = try MediaItem(movie(cast: .array([person])), server: server)
        #expect(item.cast?.first?.name == "Amy Adams")
        #expect(item.cast?.first?.role == "Louise Banks")
        #expect(item.cast?.first?.image == "/person/movie/0")
        #expect(try MediaItem(item.json, server: server).cast == item.cast)
        #expect(try JSONDecoder().decode(MediaItem.self, from: JSONEncoder().encode(item)).cast == item.cast)
        var legacy = try JSONDecoder().decode(JSONValue.self, from: JSONEncoder().encode(item)).object()
        legacy.removeValue(forKey: "cast")
        let saved = try JSONDecoder().decode(MediaItem.self, from: JSONEncoder().encode(JSONValue.object(legacy)))
        #expect(saved.cast == nil)
        #expect(try MediaItem(saved.json, server: server).id == item.id)
    }

    @Test func absentAndEmptyCastRemainValid() throws {
        let server = try ServerAddress("https://media.example")
        for cast in [JSONValue.null, .array([])] {
            #expect(try MediaItem(movie(cast: cast), server: server).cast?.isEmpty == true)
        }
        #expect(try MediaItem(.object(["id": .string("movie"), "kind": .string("video"), "title": .string("Arrival")]), server: server).cast?.isEmpty == true)
    }

    @Test func showUsesItsCastAndPreservesEpisodeOrdering() async throws {
        let cast = JSONValue.array([.object(["name": .string("Show actor"), "image": .string("/person/episode/0?scope=show")])])
        let fixture = try HTTPFixture(body: encode(show(cast: cast)))
        defer { fixture.remove() }
        let detail = try await fixture.client.show(id: showID)
        #expect(detail.cast.first?.name == "Show actor")
        #expect(detail.cast.first?.image == "/person/episode/0?scope=show")
        #expect(detail.episodes.map(\.episode) == [1, 2])
        #expect(try await fixture.client.episodes(showID: showID).map(\.id) == detail.episodes.map(\.id))
    }

    @Test func malformedCastIsRejectedForMoviesAndShowsWithoutWrites() async throws {
        let invalid: [JSONValue] = [
            .bool(true), .array([.object([:])]), .array([.object(["name": .string(" ")])]),
            .array([.object(["name": .number(2)])]),
            .array([.object(["name": .string("Actor"), "unknown": .bool(true)])]),
            .array([.object(["name": .string(String(repeating: "a", count: 257))])]),
            .array([.object(["name": .string("Actor"), "role": .string(String(repeating: "r", count: 513))])]),
            .array([.object(["name": .string("Actor"), "image": .string("https://other.example/person/a/0")])]),
            .array([.object(["name": .string("Actor"), "image": .bool(true)])]),
            .array(Array(repeating: person, count: 257))
        ]
        for cast in invalid {
            let server = try ServerAddress("https://media.example")
            #expect(throws: ClientError.self) { try MediaItem(movie(cast: cast), server: server) }
            let fixture = try HTTPFixture(body: encode(show(cast: cast)))
            defer { fixture.remove() }
            await #expect(throws: ClientError.self) { try await fixture.client.show(id: showID) }
            #expect(fixture.requests.count == 1)
            #expect(fixture.requests.allSatisfy { $0.httpMethod == "GET" })
        }
    }

    @Test func actorNormalizesNamesAndRetainsLibraryCredits() async throws {
        let fixture = try HTTPFixture(body: encode(actor()))
        defer { fixture.remove() }
        let result = try await fixture.client.actor(name: " Amy   Adams ")
        #expect(result.name == "Amy Adams")
        #expect(result.movies.first?.title == "Arrival")
        #expect(result.movies.first?.role == "Louise Banks")
        #expect(result.shows.first?.id == showID)
        #expect(fixture.requests.first?.url?.path == "/api/v1/actor")
        #expect(URLComponents(url: fixture.requests[0].url!, resolvingAgainstBaseURL: false)?.queryItems == [URLQueryItem(name: "name", value: "Amy Adams")])
        #expect(try Input.actorName(" Zoe\u{301} ") == "Zoé")
        #expect(try Input.actorName("A & B / C") == "A & B / C")
    }

    @Test func invalidActorNamesAndShowIDsDoNotReachNetwork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        for name in ["", "   ", "Amy\nAdams", "Amy\tAdams", "bad\u{0000}", String(repeating: "a", count: 201)] {
            await #expect(throws: ClientError.self) { try await fixture.client.actor(name: name) }
        }
        for id in ["", "../shows", "unknown", String(repeating: "a", count: 17)] {
            await #expect(throws: ClientError.self) { try await fixture.client.show(id: id) }
        }
        #expect(fixture.requests.isEmpty)
    }

    @Test func invalidActorResponsesAreRejectedWithoutWrites() async throws {
        var valid = try actor().object()
        var invalid: [JSONValue] = []
        for (key, value) in [
            ("name", JSONValue.string("Another actor")), ("image", .string("https://other.example/photo")),
            ("unknown", .bool(true)), ("movies", .bool(true)),
            ("movies", .array(Array(repeating: credit(), count: 10_001))),
            ("movies", .array([credit(), credit()]))
        ] { var fields = valid; fields[key] = value; invalid.append(.object(fields)) }
        valid.removeValue(forKey: "movies"); invalid.append(.object(valid))
        for (key, value) in [
            ("id", JSONValue.string("../movie")), ("title", .string("")), ("year", .bool(true)),
            ("role", .bool(true)), ("unknown", .bool(true)),
            ("url", .string("/show/movie")), ("url", .string("https://other.example/watch/movie")),
            ("artwork", .string("https://other.example/art/movie"))
        ] {
            var fields = try credit().object(); fields[key] = value
            var page = try actor().object(); page["movies"] = .array([.object(fields)])
            invalid.append(.object(page))
        }
        for raw in invalid {
            let fixture = try HTTPFixture(body: encode(raw))
            defer { fixture.remove() }
            await #expect(throws: ClientError.self) { try await fixture.client.actor(name: "Amy Adams") }
            #expect(fixture.requests.count == 1)
            #expect(fixture.requests.allSatisfy { $0.httpMethod == "GET" })
        }
    }

    @Test func emptyActorLibraryIsAValidEmptyState() throws {
        let result = try ActorDetail(.object(["name": .string("Actor"), "movies": .array([]), "shows": .array([])]),
                                     name: "Actor", server: ServerAddress("https://media.example"))
        #expect(result.movies.isEmpty && result.shows.isEmpty && result.image.isEmpty)
    }

    @Test func acceptsCombinedShowRolesWithinTheResponseBudget() throws {
        let role = ["a", "b", "c"].map { String(repeating: $0, count: 200) }.joined(separator: " · ")
        var page = try actor().object()
        page["shows"] = .array([.object(["id": .string(showID), "title": .string("Show"),
                                         "url": .string("/show/\(showID)"), "role": .string(role)])])
        let result = try ActorDetail(.object(page), name: "Amy Adams", server: ServerAddress("https://media.example"))
        #expect(result.shows.first?.role == role)
        var oversized = try credit().object()
        oversized["role"] = .string(String(repeating: "r", count: 2 * 1024 * 1024 + 1))
        page["movies"] = .array([.object(oversized)])
        #expect(throws: ClientError.self) {
            try ActorDetail(.object(page), name: "Amy Adams", server: ServerAddress("https://media.example"))
        }
    }

    @Test func actorCacheSurvivesRestartAndRejectedResponsesAreNotSaved() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true),
                               "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let fixture = try HTTPFixture(body: encode(actor()), viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.actor(name: "Amy Adams", policy: .automatic)
        await fixture.client.close()
        let reopened = try ServerClient(server: fixture.client.server, viewer: viewer,
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        let saved = try await reopened.actor(name: "Amy Adams", policy: .cached)
        #expect(saved.movies.first?.title == "Arrival")
        #expect(fixture.requests.count == 1)
        // The fixture returns Amy Adams for another actor: reject the mismatch before caching.
        await #expect(throws: ClientError.self) { try await reopened.actor(name: "Someone else") }
        await #expect(throws: CatalogCacheMiss.self) { try await reopened.actor(name: "Someone else", policy: .cached) }
        #expect(fixture.requests.count == 2)
        #expect(fixture.requests.allSatisfy { $0.httpMethod == "GET" })
        await reopened.close()
    }

    private func movie(cast: JSONValue) -> JSONValue {
        .object(["id": .string("movie"), "kind": .string("video"), "title": .string("Arrival"), "cast": cast])
    }
    private func show(cast: JSONValue) -> JSONValue {
        .object(["id": .string(showID), "title": .string("Show"), "cast": cast, "episodes": .array([2, 1].map { index in
            .object(["id": .string("episode\(index)"), "kind": .string("video"), "title": .string("Episode \(index)"),
                     "showId": .string(showID), "season": .number(1), "episode": .number(Double(index)), "cast": .array([person])])
        })])
    }
    private func credit() -> JSONValue {
        .object(["id": .string("movie"), "title": .string("Arrival"), "role": .string("Louise Banks"),
                 "artwork": .string("/art/movie"), "url": .string("/watch/movie")])
    }
    private func actor() -> JSONValue {
        .object(["name": .string("Amy Adams"), "image": .string("/person/movie/0"), "movies": .array([credit()]),
                 "shows": .array([.object(["id": .string(showID), "title": .string("Show"), "url": .string("/show/\(showID)")])])])
    }
    private func encode(_ value: JSONValue) throws -> String { String(decoding: try JSONEncoder().encode(value), as: UTF8.self) }
}
