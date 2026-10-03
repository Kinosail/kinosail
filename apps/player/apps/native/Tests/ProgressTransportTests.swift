import Foundation
import Testing
@testable import KinosailPlayer

struct ProgressTransportTests {
    @Test(arguments: [false, true])
    func progressSyncUsesTheServersFourFieldSnapshots(_ dismissed: Bool) async throws {
        let fixture = try HTTPFixture(body: """
        {"seconds":12,"watched":false,"session":"native-playback","revision":1}
        """)
        defer { fixture.remove() }
        var progress = WatchProgress()
        progress.seconds = 12; progress.session = "native-playback"; progress.revision = 1; progress.dismissed = dismissed
        var baseline = WatchProgress(); baseline.dismissed = dismissed
        let result = try await fixture.client.syncProgress(itemID: "movie", progress: progress, expected: baseline, playbackToken: "")
        #expect(!result.conflict && result.progress.seconds == 12)
        let request = try #require(fixture.requests.first)
        let body = try StrictJSON.decode(requestBody(request)).object()
        // The Server's progress/sync contract excludes locally persisted dismissal state.
        #expect(body["progress"] == .object(["seconds": .number(12), "watched": .bool(false),
                                            "session": .string("native-playback"), "revision": .number(1)]))
        #expect(body["expected"] == .object(["seconds": .number(0), "watched": .bool(false),
                                            "session": .string(""), "revision": .number(0)]))
        #expect(fixture.requests.count == 1)
    }

    @Test(arguments: ["missing session", "missing revision", "oversized session", "control character",
                      "negative position", "infinite position", "negative revision", "oversized revision"])
    func invalidSnapshotsAreRejectedBeforeNetwork(_ invalid: String) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        var valid = WatchProgress(); valid.session = "native-playback"; valid.revision = 1
        var broken = valid
        switch invalid {
        case "missing session": broken.session = ""
        case "missing revision": broken.revision = 0
        case "oversized session": broken.session = String(repeating: "s", count: 129)
        case "control character": broken.session = "session\n"
        case "negative position": broken.seconds = -1
        case "infinite position": broken.seconds = .infinity
        case "negative revision": broken.revision = -1
        default: broken.revision = 9_007_199_254_740_992
        }
        await #expect(throws: ClientError.self) {
            try await fixture.client.syncProgress(itemID: "movie", progress: broken, expected: valid, playbackToken: "")
        }
        if invalid != "missing session" && invalid != "missing revision" {
            await #expect(throws: ClientError.self) {
                try await fixture.client.syncProgress(itemID: "movie", progress: valid, expected: broken, playbackToken: "")
            }
        }
        #expect(fixture.requests.isEmpty)
    }

    private func requestBody(_ request: URLRequest) throws -> Data {
        if let body = request.httpBody { return body }
        let stream = try #require(request.httpBodyStream)
        stream.open()
        defer { stream.close() }
        var data = Data()
        var buffer = [UInt8](repeating: 0, count: 4096)
        while true {
            let count = stream.read(&buffer, maxLength: buffer.count)
            if count == 0 { return data }
            guard count > 0 else { throw stream.streamError ?? ClientError.invalidResponse }
            data.append(contentsOf: buffer.prefix(count))
            guard data.count <= 128 * 1024 else { throw ClientError.invalidResponse }
        }
    }
}
