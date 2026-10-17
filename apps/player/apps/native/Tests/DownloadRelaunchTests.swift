#if os(iOS)
import CryptoKit
import Foundation
import Testing
@testable import KinosailPlayer

struct DownloadRelaunchTests {
    @Test func keepsVerifiedBlocksFromPartialFileWhenJournalIsMissing() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let block = Data(repeating: 7, count: 8 * 1024 * 1024)
        let tail = Data(repeating: 9, count: 100)
        let digest = { (data: Data) in SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined() }
        let id = "abcdef0123456789", key = String(repeating: "b", count: 64)
        let manifest: [String: Any] = ["version": 1, "id": id, "size": block.count + tail.count, "sha256": digest(block + tail),
                                       "chunkSize": block.count, "chunks": [digest(block), digest(tail)]]
        let path = "/api/v1/downloads/" + id
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes[path + "/manifest"] = .init(data: try! JSONSerialization.data(withJSONObject: manifest), status: 200, headers: [:])
            $0[fixture.host]?.routes[path + "/file"] = .init(data: Data(), status: 200, headers: [:], hold: true)
        }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [FixtureURLProtocol.self]
        let engine = VerifiedDownloads(directory: directory, configuration: configuration)
        let access = try DownloadAuthorization(server: ServerAddress("https://" + fixture.host), serverID: "server", profileID: "viewer", token: "fixture-token")
        try await engine.authorize(access)
        let scopeDirectory = directory.resolvingSymlinksInPath().appendingPathComponent(access.scope, isDirectory: true)
        try FileManager.default.createDirectory(at: scopeDirectory, withIntermediateDirectories: true)
        try block.write(to: scopeDirectory.appendingPathComponent(key + ".media.part"))
        try await engine.enqueuePreparation(scope: access.scope, key: key, uri: "https://" + fixture.host + path + "/file", kind: "video", wifiOnly: false, quota: 0)
        var bytes: Int64 = 0
        for _ in 0..<400 where bytes < Int64(block.count) {
            try await Task.sleep(for: .milliseconds(25))
            bytes = try await engine.snapshot(access.scope).first { $0.key == key }?.bytes ?? 0
        }
        #expect(bytes >= Int64(block.count))
        await engine.close()
    }
}
#endif
