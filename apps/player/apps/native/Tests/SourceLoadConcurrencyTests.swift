import Foundation
import Testing
@testable import KinosailPlayer

@Suite(.serialized)
struct SourceLoadConcurrencyTests {
    private static func plexSections(_ types: [String]) -> [String: Any] {
        ["MediaContainer": ["Directory": types.enumerated().map { ["key": String($0.offset + 1), "type": $0.element] }]]
    }

    private static func plexRows(_ rows: [[String: Any]]) -> [String: Any] {
        ["MediaContainer": ["Metadata": rows, "totalSize": rows.count]]
    }

    private static func section(_ path: String) -> String? {
        let parts = path.split(separator: "/")
        return parts.count == 4 && parts[0] == "library" && parts[1] == "sections" && parts[3] == "all" ? String(parts[2]) : nil
    }

    @Test func plexLibraryLoadsSectionsTogetherAndKeepsTheirOrder() async throws {
        let fixture = try await SourceLoopbackFixture(barrier: 3) { request in
            if request.path == "/library/sections" { return .init(body: Self.plexSections(["movie", "movie", "show"])) }
            guard let key = Self.section(request.path) else { return .init(status: 404) }
            let type = key == "3" ? "show" : "movie"
            return .init(body: Self.plexRows([["ratingKey": "\(key)0", "type": type, "title": "Title \(key)"]]),
                         held: true, delayMilliseconds: key == "1" ? 300 : 0)
        }
        defer { fixture.close() }
        let items = try await PlexClient(account: fixture.account(.plex)).topLevel()
        #expect(fixture.maximumHeld == 3)
        #expect(items.map(\.title) == ["Title 1", "Title 2", "Title 3"])
    }

    @Test func plexContinueWatchingLoadsSectionsTogether() async throws {
        let fixture = try await SourceLoopbackFixture(barrier: 2) { request in
            if request.path == "/library/sections" { return .init(body: Self.plexSections(["movie", "show"])) }
            guard let key = Self.section(request.path), request.query["sort"] == "lastViewedAt:desc" else { return .init(status: 404) }
            let row: [String: Any] = ["ratingKey": "\(key)0", "type": key == "1" ? "movie" : "episode", "title": "Title \(key)",
                                      "grandparentRatingKey": "99", "grandparentTitle": "Fixture Show",
                                      "viewOffset": 60_000, "lastViewedAt": 1_700_000_000]
            return .init(body: Self.plexRows([row]), held: true, delayMilliseconds: key == "1" ? 300 : 0)
        }
        defer { fixture.close() }
        let items = try await PlexClient(account: fixture.account(.plex)).inProgress()
        #expect(fixture.maximumHeld == 2)
        #expect(items.map(\.title) == ["Title 1", "Title 2"])
    }

    @Test func plexLibraryFailsWhenOneSectionFails() async throws {
        let fixture = try await SourceLoopbackFixture(barrier: 2) { request in
            if request.path == "/library/sections" { return .init(body: Self.plexSections(["movie", "movie"])) }
            guard let key = Self.section(request.path) else { return .init(status: 404) }
            if key == "2" { return .init(status: 500) }
            return .init(body: Self.plexRows([["ratingKey": "10", "type": "movie", "title": "Title 1"]]), held: true)
        }
        defer { fixture.close() }
        await #expect(throws: (any Error).self) { try await PlexClient(account: fixture.account(.plex)).topLevel() }
    }

    @Test func jellyfinContinueWatchingLoadsResumeAndPlayedTogether() async throws {
        let fixture = try await SourceLoopbackFixture(barrier: 2) { request in
            switch request.path {
            case "/Users/fixtureuser/Items/Resume":
                return .init(body: ["Items": [["Id": "resume", "Type": "Movie", "Name": "Resumed"]]], held: true, delayMilliseconds: 300)
            case "/Users/fixtureuser/Items" where request.query["Filters"] == "IsPlayed":
                return .init(body: ["Items": [["Id": "played", "Type": "Movie", "Name": "Played"]]], held: true)
            default: return .init(status: 404)
            }
        }
        defer { fixture.close() }
        let items = try await JellyfinClient(account: fixture.account(.jellyfin)).inProgress()
        #expect(fixture.maximumHeld == 2)
        #expect(items.map(\.title) == ["Resumed", "Played"])
    }

    @Test func continueWatchingStartsWhileTheLibraryLoadsAndStillLinksShows() async throws {
        let fixture = try await SourceLoopbackFixture(barrier: 2) { request in
            if request.path == "/library/sections" { return .init(body: Self.plexSections(["show"])) }
            guard Self.section(request.path) == "1" else { return .init(status: 404) }
            if request.query["sort"] == "lastViewedAt:desc" {
                return .init(body: Self.plexRows([["ratingKey": "101", "type": "episode", "title": "Pilot", "grandparentRatingKey": "100",
                                                   "grandparentTitle": "Fixture Show", "viewOffset": 60_000]]), held: true)
            }
            return .init(body: Self.plexRows([["ratingKey": "100", "type": "show", "title": "Fixture Show", "Guid": [["id": "tmdb://42"]]]]),
                         held: true, delayMilliseconds: 300)
        }
        defer { fixture.close() }
        let directory = FileManager.default.temporaryDirectory.appending(path: "source-load-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: directory) }
        let catalog = DeviceCatalog(index: DeviceIndexStore(directory: directory))
        await catalog.configure([fixture.account(.plex)])
        let items = try await catalog.recentActivity()
        #expect(fixture.maximumHeld == 2)
        #expect(items.map(\.title) == ["Pilot"])
        #expect(items.first?.seriesProviders["tmdb"] == "42")
    }
}
