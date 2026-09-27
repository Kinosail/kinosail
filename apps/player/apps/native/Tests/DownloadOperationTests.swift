import Foundation
import Testing
@testable import KinosailPlayer

struct DownloadOperationTests {
    @Test func validatesDownloadInputsBeforeNetwork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.prepareDownload(itemID: "../movie", quality: .original) }
        await #expect(throws: ClientError.self) { try await fixture.client.prepareDownload(itemID: "movie", quality: .original, tracks: .init(audio: [], subtitles: [])) }
        await #expect(throws: ClientError.self) { try await fixture.client.prepareDownload(itemID: "movie", quality: .compatible, tracks: .init(audio: [0, 0], subtitles: [])) }
        await #expect(throws: ClientError.self) { try await fixture.client.prepareDownload(itemID: "movie", quality: .compatible, tracks: .init(audio: [-1], subtitles: [])) }
        await #expect(throws: ClientError.self) { try await fixture.client.prepareDownload(itemID: "movie", quality: .compatible, tracks: .init(audio: [0], subtitles: [4096])) }
        await #expect(throws: ClientError.self) { try await fixture.client.preparedDownload(id: "../manifest") }
        await #expect(throws: ClientError.self) { try await fixture.client.downloadManifest(id: String(repeating: "a", count: 17)) }
        #expect(fixture.requests.isEmpty)
    }

    @Test func acceptsAudioPreparationAndRejectsConflictingReadyState() throws {
        var value: [String: JSONValue] = ["id": .string(String(repeating: "a", count: 16)), "itemId": .string("audio-item"), "profileId": .string("viewer"),
            "title": .string("Track"), "quality": .string("audio"), "state": .string("preparing"), "readyOffline": .bool(false), "created": .string("2026-09-10T12:00:00Z")]
        let prepared = try PreparedDownload(.object(value), profileID: "viewer")
        #expect(prepared.quality == .audio)
        value["readyOffline"] = .bool(true)
        #expect(throws: ClientError.self) { try PreparedDownload(.object(value), profileID: "viewer") }
        value["readyOffline"] = .bool(false)
        #expect(throws: ClientError.self) { try PreparedDownload(.object(value), profileID: "other-viewer") }
        value["extra"] = .string("unknown")
        #expect(throws: ClientError.self) { try PreparedDownload(.object(value), profileID: "viewer") }
    }

    @Test func boundsManifestBeforeAnyTransfer() throws {
        let hash = String(repeating: "a", count: 64)
        let valid: [String: JSONValue] = ["version": .number(1), "id": .string(String(repeating: "b", count: 16)), "size": .number(10), "sha256": .string(hash), "chunkSize": .number(8_388_608), "chunks": .array([.string(hash)])]
        #expect(try OfflineManifest(.object(valid)).length(0) == 10)
        for (key, bad) in [("version", JSONValue.number(2)), ("size", .number(0)), ("size", .number(137_438_953_473)), ("size", .number(1.5)),
                           ("chunkSize", .number(1)), ("sha256", .string("bad")), ("chunks", .array([])), ("chunks", .array([.bool(true)])), ("chunks", .array([.string(hash), .string(hash)]))] {
            var value = valid; value[key] = bad
            #expect(throws: ClientError.self) { try OfflineManifest(.object(value)) }
        }
    }

    #if os(iOS)
    @MainActor @Test func rejectsInvalidSeriesBeforeAnyDownloadRequest() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let manager = OfflineDownloadManager()
        let server = try ServerAddress("https://\(fixture.host)")
        func episode(_ id: String, show: String = "aaaaaaaaaaaaaaaa", season: Int = 1, kind: String = "video") throws -> MediaItem {
            try MediaItem(.object(["id": .string(id), "kind": .string(kind), "title": .string(id),
                                   "showId": .string(show), "season": .number(Double(season)), "episode": .number(1)]), server: server)
        }
        let first = try episode("first")
        let second = try episode("second", season: 2)
        let foreign = try episode("foreign", show: "bbbbbbbbbbbbbbbb")
        let audio = try episode("audio", kind: "music")
        let missingShow = try episode("missing", show: "")
        let oversized = try (0..<51).map { try episode("episode-\($0)") }
        for items in [[], [missingShow], [first, first], [first, foreign], [first, audio], oversized] {
            await #expect(throws: ClientError.invalidInput("Choose up to 50 unique video episodes from one series.")) {
                try await manager.enqueueSeries(items, quality: .compatible, client: fixture.client)
            }
        }
        await #expect(throws: ClientError.invalidInput("Choose up to 50 unique video episodes from one series.")) {
            try await manager.enqueueSeries([first, second], quality: .audio, client: fixture.client)
        }
        await #expect(throws: ClientError.invalidInput("Wait for the current download operation to finish.")) {
            try await manager.enqueueSeries([first, second], quality: .compatible, client: fixture.client)
        }
        #expect(fixture.requests.isEmpty)
        #expect(manager.downloads.isEmpty)
    }

    @Test func identifiesExtensionlessMediaWithoutAcceptingPlaylists() throws {
        #expect(try OfflineProbe.contentType(Data([0, 0, 0, 24] + Array("ftypisom".utf8))) == "video/mp4")
        #expect(try OfflineProbe.contentType(Data("RIFF0000WAVE".utf8)) == "audio/wav")
        #expect(try OfflineProbe.contentType(Data("fLaC00000000".utf8)) == "audio/flac")
        for bytes in [Data(), Data("ftyp".utf8), Data(repeating: 0, count: 65), Data("#EXTM3U\nhttps://example.com/media".utf8), Data("RIFF0000AVI ".utf8), Data(repeating: 0, count: 64)] {
            #expect(throws: ClientError.self) { try OfflineProbe.contentType(bytes) }
        }
        #expect(throws: ClientError.self) { try OfflineProbe.asset(URL(string: "https://example.com/movie.mp4")!) }
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let link = directory.appendingPathComponent("media")
        try FileManager.default.createSymbolicLink(at: link, withDestinationURL: directory.appendingPathComponent("missing"))
        #expect(throws: (any Error).self) { try OfflineProbe.asset(link) }
        #expect(try FileManager.default.contentsOfDirectory(atPath: directory.path) == ["media"])
    }

    @Test func rejectsPreparationAndForeignOriginWithoutCreatingFiles() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [FixtureURLProtocol.self]
        let engine = VerifiedDownloads(directory: directory, configuration: configuration)
        let access = try DownloadAuthorization(server: ServerAddress("https://media.example"), serverID: "server", profileID: "viewer", token: "fixture-token")
        try await engine.authorize(access)
        let key = String(repeating: "c", count: 64)
        let path = "/api/v1/downloads/" + String(repeating: "d", count: 16) + "/file"
        await #expect(throws: ClientError.self) { try await engine.enqueuePreparation(scope: access.scope, key: "../escape", uri: "https://media.example" + path, kind: "video", wifiOnly: true, quota: 0) }
        await #expect(throws: ClientError.self) { try await engine.enqueuePreparation(scope: access.scope, key: key, uri: "https://other.example" + path, kind: "video", wifiOnly: true, quota: 0) }
        await #expect(throws: ClientError.self) { try await engine.enqueuePreparation(scope: access.scope, key: key, uri: "https://media.example" + path, kind: "video", wifiOnly: true, quota: -1) }
        #expect(!FileManager.default.fileExists(atPath: directory.path))
        #expect(try await engine.snapshot(access.scope).isEmpty)
        await engine.close()
    }
    #endif
}
