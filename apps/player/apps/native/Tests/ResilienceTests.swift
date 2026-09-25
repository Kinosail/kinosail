import Foundation
import Testing
@testable import KinosailPlayer

struct ResilienceTests {
    @Test func offlineRestoreKeepsAuthorizationFailuresVisible() {
        for error in [ClientError.unavailable, .http(408), .http(429), .http(500), .http(502), .http(503), .http(504)] {
            #expect(error.permitsOfflineRestore)
        }
        for error in [ClientError.http(401), .http(403), .http(404), .http(302), .invalidResponse, .secureStorage] {
            #expect(!error.permitsOfflineRestore)
        }
    }

    @Test func retryBudgetAndJitterHonorServerMinimum() throws {
        let url = URL(string: "https://media.example")!
        let response = HTTPURLResponse(url: url, statusCode: 503, httpVersion: nil, headerFields: ["Retry-After": "60"])!
        #expect(DownloadRecovery.delay(error: nil, response: response, attempt: 0, jitter: 0) == 60)
        #expect(DownloadRecovery.delay(error: nil, response: response, attempt: 0, jitter: 1) == 75)
        #expect(DownloadRecovery.delay(error: nil, response: response, attempt: 5) == nil)
        #expect(DownloadRecovery.delay(error: nil, response: response, attempt: -1) == nil)
        #expect(DownloadRecovery.delay(error: nil, response: response, attempt: 0, jitter: .nan) == nil)
        for status in [401, 403, 404, 412, 416] {
            let denied = HTTPURLResponse(url: url, statusCode: status, httpVersion: nil, headerFields: [:])!
            #expect(DownloadRecovery.delay(error: URLError(.timedOut) as NSError, response: denied, attempt: 0) == nil)
        }
        for raw in ["-1", "3601", "NaN", String(repeating: "1", count: 65)] {
            let invalid = HTTPURLResponse(url: url, statusCode: 503, httpVersion: nil, headerFields: ["Retry-After": raw])!
            #expect(DownloadRecovery.delay(error: nil, response: invalid, attempt: 0) == nil)
        }
    }

    @Test func oneMissingTitleDoesNotBlockOtherProgress() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let viewer = try Viewer(.object(["server": .string("Fixture"), "serverId": .string("server"), "viewer": .object([
            "id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true), "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        try await fixture.client.associate(viewer)
        let scope = try await fixture.client.profileScope()
        let store = try ProgressSyncStore(scope: scope, directory: root)
        var progress = WatchProgress(); progress.seconds = 120; progress.session = "session"; progress.revision = 1
        try await store.record(itemID: "missing", progress: progress, expected: WatchProgress())
        try await store.record(itemID: "available", progress: progress, expected: WatchProgress())
        let success = try JSONEncoder().encode(progress.json)
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/api/v1/items/missing/progress/sync"] = .init(data: Data("{}".utf8), status: 404, headers: [:])
            $0[fixture.host]?.routes["/api/v1/items/available/progress/sync"] = .init(data: success, status: 200, headers: [:])
        }
        let result = try await store.synchronize(client: fixture.client)
        #expect(result["available"]?.conflict == false)
        #expect(try await store.pending().map(\.itemID) == ["missing"])
        #expect(fixture.requests.count == 2)
    }

    #if os(iOS)
    @Test func downloadsAndVerifiesPlayableAudioBeforePublishing() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let id = "aaaaaaaaaaaaaaaa", key = String(repeating: "b", count: 64)
        var wave = Data("RIFF".utf8)
        func append16(_ value: Int) { wave.append(contentsOf: [UInt8(value & 255), UInt8((value >> 8) & 255)]) }
        func append32(_ value: Int) { for shift in [0, 8, 16, 24] { wave.append(UInt8((value >> shift) & 255)) } }
        append32(16_036); wave.append(contentsOf: Data("WAVEfmt ".utf8))
        append32(16); append16(1); append16(1); append32(8_000); append32(16_000); append16(2); append16(16)
        wave.append(contentsOf: Data("data".utf8)); append32(16_000); wave.append(Data(repeating: 0, count: 16_000))
        let digest = VerifiedDownload.hash(wave)
        let manifest = JSONValue.object(["version": .number(1), "id": .string(id), "size": .number(Double(wave.count)),
            "sha256": .string(digest), "chunkSize": .number(8 * 1024 * 1024), "chunks": .array([.string(digest)])])
        let manifestData = try JSONEncoder().encode(manifest)
        let path = "/api/v1/downloads/" + id
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes[path + "/manifest"] = .init(data: manifestData, status: 200, headers: [:])
            $0[fixture.host]?.routes[path + "/file"] = .init(data: wave, status: 206, headers: [
                "ETag": "\"\(digest)\"", "Content-Range": "bytes 0-\(wave.count - 1)/\(wave.count)"])
        }
        let configuration = URLSessionConfiguration.ephemeral; configuration.protocolClasses = [FixtureURLProtocol.self]
        let engine = VerifiedDownloads(directory: root, configuration: configuration)
        let access = try DownloadAuthorization(server: ServerAddress("https://" + fixture.host), serverID: "server", profileID: "viewer", token: "fixture-token")
        try await engine.authorize(access)
        try await engine.enqueuePreparation(scope: access.scope, key: key, uri: "https://" + fixture.host + path + "/file", kind: "audio", wifiOnly: false, quota: 0)
        let deadline = Date().addingTimeInterval(10)
        var snapshot = try await engine.snapshot(access.scope).first { $0.key == key }
        while snapshot?.status != "complete" && snapshot?.status != "paused" && Date() < deadline {
            try await Task.sleep(for: .milliseconds(50))
            snapshot = try await engine.snapshot(access.scope).first { $0.key == key }
        }
        #expect(snapshot?.status == "complete")
        #expect(fixture.requests.map { $0.url?.path } == [path + "/manifest", path + "/file"])
        #expect(fixture.requests.last?.value(forHTTPHeaderField: "Range") == "bytes=0-\(wave.count - 1)")
        let file = try await engine.file(access.scope, key: key)
        #expect(try Data(contentsOf: file) == wave)
        await engine.close()
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.hold = true }
        let reopened = VerifiedDownloads(directory: root, configuration: configuration)
        try await reopened.authorize(access)
        try await reopened.check(access.scope, key: key)
        let reopenedDeadline = Date().addingTimeInterval(10)
        var reopenedStatus = try await reopened.snapshot(access.scope).first?.status
        while reopenedStatus != "complete" && reopenedStatus != "paused" && Date() < reopenedDeadline {
            try await Task.sleep(for: .milliseconds(50))
            reopenedStatus = try await reopened.snapshot(access.scope).first?.status
        }
        #expect(reopenedStatus == "complete")
        let reopenedFile = try await reopened.file(access.scope, key: key)
        #expect(try Data(contentsOf: reopenedFile) == wave)
        #expect(fixture.requests.count == 2)
        await reopened.close()
    }

    @Test func pausedExtentsLeaveRoomForAnotherPreparation() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let firstID = "aaaaaaaaaaaaaaaa", secondID = "bbbbbbbbbbbbbbbb"
        let manifest = JSONValue.object(["version": .number(1), "id": .string(firstID), "size": .number(128 * 1024 * 1024),
            "sha256": .string(String(repeating: "a", count: 64)), "chunkSize": .number(8 * 1024 * 1024),
            "chunks": .array(Array(repeating: .string(String(repeating: "a", count: 64)), count: 16))])
        let encoded = try JSONEncoder().encode(manifest)
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/api/v1/downloads/" + firstID + "/manifest"] = .init(data: encoded, status: 200, headers: [:])
            $0[fixture.host]?.routes["/api/v1/downloads/" + firstID + "/file"] = .init(data: Data(), status: 200, headers: [:], hold: true)
            $0[fixture.host]?.routes["/api/v1/downloads/" + secondID + "/manifest"] = .init(data: Data(), status: 200, headers: [:], hold: true)
        }
        let configuration = URLSessionConfiguration.ephemeral; configuration.protocolClasses = [FixtureURLProtocol.self]
        let engine = VerifiedDownloads(directory: root, configuration: configuration)
        let access = try DownloadAuthorization(server: ServerAddress("https://" + fixture.host), serverID: "server", profileID: "viewer", token: "fixture-token")
        try await engine.authorize(access)
        let firstKey = String(repeating: "a", count: 64), secondKey = String(repeating: "b", count: 64)
        try await engine.enqueuePreparation(scope: access.scope, key: firstKey, uri: "https://" + fixture.host + "/api/v1/downloads/" + firstID + "/file", kind: "video", wifiOnly: false, quota: 0)
        var deadline = Date().addingTimeInterval(3)
        while fixture.requests.filter({ $0.url?.path.hasSuffix("/file") == true }).count < 2, Date() < deadline { try await Task.sleep(for: .milliseconds(10)) }
        #expect(fixture.requests.filter { $0.url?.path.hasSuffix("/file") == true }.count == 2)
        try await engine.pause(access.scope, key: firstKey)
        try await engine.enqueuePreparation(scope: access.scope, key: secondKey, uri: "https://" + fixture.host + "/api/v1/downloads/" + secondID + "/file", kind: "video", wifiOnly: false, quota: 0)
        deadline = Date().addingTimeInterval(3)
        while !fixture.requests.contains(where: { $0.url?.path.contains(secondID) == true }), Date() < deadline { try await Task.sleep(for: .milliseconds(10)) }
        #expect(fixture.requests.contains { $0.url?.path.contains(secondID) == true })
        #expect(try await engine.snapshot(access.scope).first(where: { $0.key == firstKey })?.status == "paused")
        await engine.close()
    }

    @Test func missingPayloadClearsStaleVerifiedBitsOnResume() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let access = try DownloadAuthorization(server: ServerAddress("https://" + fixture.host), serverID: "server", profileID: "viewer", token: "fixture-token")
        let key = String(repeating: "a", count: 64), id = "aaaaaaaaaaaaaaaa"
        let manifest = try OfflineManifest(.object(["version": .number(1), "id": .string(id), "size": .number(12), "sha256": .string(key), "chunkSize": .number(8 * 1024 * 1024), "chunks": .array([.string(key)])]))
        let job = VerifiedDownload(scope: access.scope, key: key, uri: "https://" + fixture.host + "/api/v1/downloads/" + id + "/file", manifest: manifest, kind: "video", wifiOnly: false, verified: [true], status: "complete")
        let folder = root.appendingPathComponent(access.scope)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        try JSONEncoder().encode(job).write(to: folder.appendingPathComponent(key + ".transfer"))
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.hold = true }
        let configuration = URLSessionConfiguration.ephemeral; configuration.protocolClasses = [FixtureURLProtocol.self]
        let engine = VerifiedDownloads(directory: root, configuration: configuration)
        try await engine.authorize(access)
        try await engine.resume(access.scope, key: key, wifiOnly: false, quota: 0)
        let deadline = Date().addingTimeInterval(3)
        while fixture.requests.isEmpty, Date() < deadline { try await Task.sleep(for: .milliseconds(10)) }
        #expect(fixture.requests.first?.value(forHTTPHeaderField: "Range") == "bytes=0-11")
        #expect(try await engine.snapshot(access.scope).first?.bytes == 0)
        await engine.close()
    }

    @Test func corruptJournalDoesNotDisableHealthyDownloadsAndDeletionRecovers() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let access = try DownloadAuthorization(server: ServerAddress("https://media.example"), serverID: "server", profileID: "viewer", token: "fixture-token")
        let folder = root.appendingPathComponent(access.scope)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let key = String(repeating: "a", count: 64)
        let plan = DownloadPreparation(scope: access.scope, key: key, uri: "https://media.example/api/v1/downloads/aaaaaaaaaaaaaaaa/file", kind: "video", wifiOnly: true, quota: 0, status: "paused", error: "")
        try JSONEncoder().encode(plan).write(to: folder.appendingPathComponent(key + ".planning"))
        let bad = String(repeating: "b", count: 64)
        try Data("invalid".utf8).write(to: folder.appendingPathComponent(bad + ".transfer"))
        let removed = String(repeating: "c", count: 64)
        for suffix in [".deleting", ".media", ".media.part", ".planning"] {
            try Data("unfinished removal".utf8).write(to: folder.appendingPathComponent(removed + suffix))
        }
        let engine = VerifiedDownloads(directory: root, configuration: .ephemeral)
        try await engine.authorize(access)
        #expect(try await engine.snapshot(access.scope).map(\.key) == [key])
        #expect(FileManager.default.fileExists(atPath: folder.appendingPathComponent(bad + ".transfer.invalid").path))
        for suffix in [".deleting", ".media", ".media.part", ".planning"] {
            #expect(!FileManager.default.fileExists(atPath: folder.appendingPathComponent(removed + suffix).path))
        }
        try await engine.remove(access.scope, key: bad)
        #expect(!FileManager.default.fileExists(atPath: folder.appendingPathComponent(bad + ".transfer.invalid").path))
        await engine.close()
    }

    @Test func verificationCacheRejectsSameSizeRewriteAndSymlink() throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        let file = root.appendingPathComponent("media")
        try Data("original".utf8).write(to: file)
        let before = try DownloadFileStamp(file)
        try Data("modified".utf8).write(to: file, options: .atomic)
        #expect(before != (try DownloadFileStamp(file)))
        let link = root.appendingPathComponent("link")
        try FileManager.default.createSymbolicLink(at: link, withDestinationURL: file)
        #expect(throws: ClientError.self) { try DownloadFileStamp(link) }
    }
    #endif
}
