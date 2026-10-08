#if os(iOS)
import Foundation
import Testing
@testable import KinosailPlayer

struct DownloadStorageAuthorizationTests {
    @Test func lockedStorageFailuresPreserveAuthorizationDenialWithoutSideEffects() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        let blocker = Data("synthetic unreadable journal root".utf8)
        try blocker.write(to: root)
        defer { try? FileManager.default.removeItem(at: root) }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [FixtureURLProtocol.self]
        let downloads = VerifiedDownloads(directory: root, configuration: configuration)
        do {
            let access = try DownloadAuthorization(server: ServerAddress("https://" + fixture.host), serverID: "server", profileID: "viewer", token: "synthetic-token")
            let revision = await downloads.authorizationRevision()
            let key = String(repeating: "b", count: 64)
            for scope in [access.scope, String(repeating: "c", count: 64), "", "../scope", String(repeating: "a", count: 4097)] {
                let operations: [@Sendable () async throws -> Void] = [
                    { _ = try await downloads.snapshot(scope) },
                    { try await downloads.pause(scope, key: key) },
                    { try await downloads.remove(scope, key: key) },
                    { try await downloads.resume(scope, key: key, wifiOnly: false, quota: 0) },
                    { _ = try await downloads.file(scope, key: key) },
                    { try await downloads.check(scope, key: key) },
                    { try await downloads.updatePolicy(scope: scope, wifiOnly: false, quota: 0) },
                    { try await downloads.enqueuePreparation(scope: scope, key: key, uri: "https://" + fixture.host + "/api/v1/downloads/aaaaaaaaaaaaaaaa/file", kind: "audio", wifiOnly: false, quota: 0) },
                ]
                for operation in operations {
                    await #expect(throws: ClientError.http(403)) { try await operation() }
                    #expect(try Data(contentsOf: root) == blocker)
                    #expect(fixture.requests.isEmpty)
                }
            }
            await #expect(throws: ClientError.invalidInput("The saved download could not be read. Remove it and try again.")) {
                try await downloads.authorize(access)
            }
            #expect(await downloads.authorizationRevision() == revision)
            #expect(try Data(contentsOf: root) == blocker)
            try await downloads.reset()
            #expect(!FileManager.default.fileExists(atPath: root.path))
            await #expect(throws: ClientError.http(403)) { try await downloads.snapshot(access.scope) }
            try await downloads.authorize(access)
            #expect(try await downloads.snapshot(access.scope).isEmpty)
            #expect(fixture.requests.isEmpty)
            print("OFFLINE storage failure: 40 scoped denials, no network or file mutation; reset requires authorization")
            await downloads.close()
        } catch {
            await downloads.close()
            throw error
        }
    }
}
#endif
