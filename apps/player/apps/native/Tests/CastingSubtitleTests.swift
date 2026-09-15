import Foundation
import Testing
@testable import KinosailPlayer

struct CastingSubtitleTests {
    @Test func rejectsCastInputsBeforeNetwork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        for input in [CastStart(receiverProtocol: .dlna, deviceID: nil, position: 0, playbackToken: nil),
                      CastStart(receiverProtocol: .dlna, deviceID: "../tv", position: 0, playbackToken: nil),
                      CastStart(receiverProtocol: .googleCast, deviceID: String(repeating: "a", count: 32), position: 0, playbackToken: nil),
                      CastStart(receiverProtocol: .googleCast, deviceID: nil, position: .nan, playbackToken: nil),
                      CastStart(receiverProtocol: .googleCast, deviceID: nil, position: 0, playbackToken: "secret\nheader"),
                      CastStart(receiverProtocol: .googleCast, deviceID: nil, position: 0, playbackToken: String(repeating: "a", count: 8193))] {
            await #expect(throws: ClientError.self) { try await fixture.client.startCast(itemID: "movie", request: input) }
        }
        await #expect(throws: ClientError.self) { try await fixture.client.castCommand(id: String(repeating: "a", count: 32), command: .seek(-1)) }
        await #expect(throws: ClientError.self) { try await fixture.client.endCast(id: "../tv") }
        #expect(fixture.requests.isEmpty)
    }

    @Test func requiresScopedCastTicketsAndMatchingDevice() throws {
        let id = String(repeating: "a", count: 32), ticket = String(repeating: "b", count: 64)
        let server = try ServerAddress("https://media.example")
        let value: [String: JSONValue] = ["id": .string(id), "url": .string("https://media.example/cast/\(id)/media?ticket=\(ticket)"),
            "contentType": .string("video/mp4"), "title": .string("Movie"), "position": .number(10), "duration": .number(100),
            "expiresAt": .string(ISO8601DateFormatter().string(from: Date().addingTimeInterval(3600))), "protocol": .string("dlna"),
            "tracks": .array([]), "deviceId": .string(String(repeating: "c", count: 32)), "deviceName": .string("Living room")]
        #expect(try CastSession(.object(value), server: server).duration == 100)
        for (key, invalid) in [("url", JSONValue.string("https://other.example/cast/\(id)/media?ticket=\(ticket)")),
                               ("url", .string("/media/movie?ticket=\(ticket)")),
                               ("url", .string("/cast/\(id)/media?ticket=\(ticket)&ticket=\(ticket)")),
                               ("position", .number(101)), ("expiresAt", .string("2000-01-01T00:00:00Z")),
                               ("protocol", .string("airplay")), ("deviceId", .string("bad")), ("contentType", .string("text/html"))] {
            var changed = value; changed[key] = invalid
            #expect(throws: ClientError.self) { try CastSession(.object(changed), server: server) }
        }
    }

    @Test func rendersOverlappingWebVTTCuesAtExactBoundaries() throws {
        let document = try SubtitleDocument(data: Data("WEBVTT\n\n1\n00:00.500 --> 00:02.000 align:center\n<v Narrator>Hello &amp; welcome</v>\n\n00:01.500 --> 00:03.000\nSecond line\n\nNOTE comment\nignored\n".utf8))
        #expect(document.text(at: 0.49).isEmpty)
        #expect(document.text(at: 0.5) == "Hello & welcome")
        #expect(document.text(at: 1.5) == "Hello & welcome\nSecond line")
        #expect(document.text(at: 2) == "Second line")
        #expect(document.text(at: 3).isEmpty)
        #expect(document.text(at: .nan).isEmpty)
    }

    @Test func boundsUntrustedCaptions() throws {
        for value in ["not vtt", "WEBVTT\n\n00:99.000 --> 00:01.000\nInvalid", "WEBVTT\n\n00:02.000 --> 00:01.000\nBackwards", "WEBVTT\n\n00:00.000 --> 00:01.000\n\0", "WEBVTT\n\n00:00.000 --> 00:01.000\n" + String(repeating: "x", count: 4097), "WEBVTT\n\n" + Array(repeating: "00:00.000 --> 00:10.000\nOverlap\n\n", count: 33).joined()] {
            #expect(throws: ClientError.self) { try SubtitleDocument(data: Data(value.utf8)) }
        }
        #expect(throws: ClientError.self) { try SubtitleDocument(data: Data(repeating: 65, count: 2 * 1024 * 1024 + 1)) }
    }
}
