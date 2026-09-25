#if os(iOS)
import Foundation
import Testing
@testable import KinosailPlayer

struct DownloadVideoRoundTripTests {
    @Test func rejectsWrongValidatorThenResumesAndPlaysVideo() async throws {
        let movie = try #require(Data(base64Encoded: """
AAAAIGZ0eXBpc29tAAACAGlzb21pc28yYXZjMW1wNDEAAAMwbW9vdgAAAGxtdmhkAAAAAAAAAAAAAAAAAAAD6AAAB9AAAQAA
AQAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAgAAAlt0cmFrAAAAXHRraGQAAAADAAAAAAAAAAAAAAABAAAAAAAAB9AAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAA
AAAAAAABAAAAAAAAAAAAAAAAAABAAAAAABAAAAAQAAAAAAAkZWR0cwAAABxlbHN0AAAAAAAAAAEAAAfQAAAAAAABAAAAAAHT
bWRpYQAAACBtZGhkAAAAAAAAAAAAAAAAAABAAAAAgABVxAAAAAAALWhkbHIAAAAAAAAAAHZpZGUAAAAAAAAAAAAAAABWaWRl
b0hhbmRsZXIAAAABfm1pbmYAAAAUdm1oZAAAAAEAAAAAAAAAAAAAACRkaW5mAAAAHGRyZWYAAAAAAAAAAQAAAAx1cmwgAAAA
AQAAAT5zdGJsAAAAvnN0c2QAAAAAAAAAAQAAAK5hdmMxAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAABAAEABIAAAASAAAAAAA
AAABFExhdmM2My4xLjEwMSBsaWJ4MjY0AAAAAAAAAAAAAAAAGP//AAAANGF2Y0MBZAAK/+EAF2dkAAqs2V7ARAAAAwAEAAAD
AAg8SJZYAQAGaOvjyyLA/fj4AAAAABBwYXNwAAAAAQAAAAEAAAAUYnRydAAAAAAAAAtEAAAAAAAAABhzdHRzAAAAAAAAAAEA
AAACAABAAAAAABRzdHNzAAAAAAAAAAEAAAABAAAAHHN0c2MAAAAAAAAAAQAAAAEAAAACAAAAAQAAABxzdHN6AAAAAAAAAAAA
AAACAAACxQAAAAwAAAAUc3RjbwAAAAAAAAABAAADYAAAAGF1ZHRhAAAAWW1ldGEAAAAAAAAAIWhkbHIAAAAAAAAAAG1kaXJh
cHBsAAAAAAAAAAAAAAAALGlsc3QAAAAkqXRvbwAAABxkYXRhAAAAAQAAAABMYXZmNjMuMS4xMDEAAAAIZnJlZQAAAtltZGF0
AAACrQYF//+p3EXpvebZSLeWLNgg2SPu73gyNjQgLSBjb3JlIDE2NSByMzIyMiBiMzU2MDVhIC0gSC4yNjQvTVBFRy00IEFW
QyBjb2RlYyAtIENvcHlsZWZ0IDIwMDMtMjAyNSAtIGh0dHA6Ly93d3cudmlkZW9sYW4ub3JnL3gyNjQuaHRtbCAtIG9wdGlv
bnM6IGNhYmFjPTEgcmVmPTMgZGVibG9jaz0xOjA6MCBhbmFseXNlPTB4MzoweDExMyBtZT1oZXggc3VibWU9NyBwc3k9MSBw
c3lfcmQ9MS4wMDowLjAwIG1peGVkX3JlZj0xIG1lX3JhbmdlPTE2IGNocm9tYV9tZT0xIHRyZWxsaXM9MSA4eDhkY3Q9MSBj
cW09MCBkZWFkem9uZT0yMSwxMSBmYXN0X3Bza2lwPTEgY2hyb21hX3FwX29mZnNldD0tMiB0aHJlYWRzPTEgbG9va2FoZWFk
X3RocmVhZHM9MSBzbGljZWRfdGhyZWFkcz0wIG5yPTAgZGVjaW1hdGU9MSBpbnRlcmxhY2VkPTAgYmx1cmF5X2NvbXBhdD0w
IGNvbnN0cmFpbmVkX2ludHJhPTAgYmZyYW1lcz0zIGJfcHlyYW1pZD0yIGJfYWRhcHQ9MSBiX2JpYXM9MCBkaXJlY3Q9MSB3
ZWlnaHRiPTEgb3Blbl9nb3A9MCB3ZWlnaHRwPTIga2V5aW50PTI1MCBrZXlpbnRfbWluPTEgc2NlbmVjdXQ9NDAgaW50cmFf
cmVmcmVzaD0wIHJjX2xvb2thaGVhZD00MCByYz1jcmYgbWJ0cmVlPTEgY3JmPTIzLjAgcWNvbXA9MC42MCBxcG1pbj0wIHFw
bWF4PTY5IHFwc3RlcD00IGlwX3JhdGlvPTEuNDAgYXE9MToxLjAwAIAAAAAQZYiEABb//vfTP8yy7JolgQAAAAhBmiFsQV/+
8A==
""", options: .ignoreUnknownCharacters))
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: root) }
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let id = "aaaaaaaaaaaaaaaa", key = String(repeating: "b", count: 64)
        let digest = VerifiedDownload.hash(movie)
        let manifest = JSONValue.object(["version": .number(1), "id": .string(id), "size": .number(Double(movie.count)),
            "sha256": .string(digest), "chunkSize": .number(8 * 1024 * 1024), "chunks": .array([.string(digest)])])
        let manifestData = try JSONEncoder().encode(manifest)
        let path = "/api/v1/downloads/" + id
        let headers = ["ETag": "\"\(digest)\"", "Content-Range": "bytes 0-\(movie.count - 1)/\(movie.count)"]
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes[path + "/manifest"] = .init(data: manifestData, status: 200, headers: [:])
            $0[fixture.host]?.routes[path + "/file"] = .init(data: movie, status: 206, headers: headers.merging(["ETag": "\"stale\""]) { _, new in new })
        }
        let configuration = URLSessionConfiguration.ephemeral; configuration.protocolClasses = [FixtureURLProtocol.self]
        let engine = VerifiedDownloads(directory: root, configuration: configuration)
        let access = try DownloadAuthorization(server: ServerAddress("https://" + fixture.host), serverID: "server", profileID: "viewer", token: "fixture-token")
        try await engine.authorize(access)
        try await engine.enqueuePreparation(scope: access.scope, key: key, uri: "https://" + fixture.host + path + "/file", kind: "video", wifiOnly: false, quota: 0)
        let badDeadline = Date().addingTimeInterval(10)
        var status = try await engine.snapshot(access.scope).first { $0.key == key }?.status
        while status != "paused" && Date() < badDeadline {
            try await Task.sleep(for: .milliseconds(50))
            status = try await engine.snapshot(access.scope).first { $0.key == key }?.status
        }
        #expect(status == "paused")
        await #expect(throws: ClientError.self) { try await engine.file(access.scope, key: key) }
        #expect(!FileManager.default.fileExists(atPath: root.appendingPathComponent(access.scope).appendingPathComponent(key + ".media").path))
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.routes[path + "/file"] = .init(data: movie, status: 206, headers: headers) }
        try await engine.resume(access.scope, key: key, wifiOnly: false, quota: 0)
        let goodDeadline = Date().addingTimeInterval(10)
        while status != "complete" && Date() < goodDeadline {
            try await Task.sleep(for: .milliseconds(50))
            status = try await engine.snapshot(access.scope).first { $0.key == key }?.status
        }
        #expect(status == "complete")
        let file = try await engine.file(access.scope, key: key)
        #expect(try Data(contentsOf: file) == movie)
        #expect(fixture.requests.filter { $0.url?.path == path + "/file" }.count == 2)
        await engine.close()
    }
}
#endif
