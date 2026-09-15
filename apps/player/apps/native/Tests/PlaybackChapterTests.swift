import Foundation
import Testing
@testable import KinosailPlayer

struct PlaybackChapterTests {
    private func source(_ chapters: String, start: String = "0") throws -> PlaybackSource {
        let body = """
        {"media":{"duration":60},"plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"duration":60,"start":\(start),"directAllowed":true,"direct":"/media/movie","directType":"video/mp4","chapters":\(chapters)}
        """
        return try PlaybackSource(StrictJSON.decode(Data(body.utf8)), itemID: "movie", server: ServerAddress("https://example.com"))
    }

    @Test func acceptsServerChapterIndexes() throws {
        let result = try source("""
        [{"index":0,"title":"Opening","start":0,"end":30},{"index":1,"title":"Ending","start":30,"end":60}]
        """)
        #expect(result.chapters.count == 2)
        #expect(result.chapters[1].start == 30)
        #expect(result.chapters[1].end == 60)
        #expect(try source("[{\"title\":\"Opening\",\"start\":0,\"end\":60}]").chapters.count == 1)
    }

    @Test func restartsStaleProgressAndPreservesValidResume() throws {
        #expect(try source("[]", start: "4771.413").start == 0)
        #expect(try source("[]", start: "60").start == 0)
        #expect(try source("[]", start: "29.5").start == 29.5)
        for invalid in ["-1", "31536001", "null", "true", "\"12\""] {
            #expect(throws: ClientError.self) { try source("[]", start: invalid) }
        }
    }

    @Test(arguments: ["-1", "1", "1024", "0.5", "true", "null", "\"0\""])
    func rejectsInvalidChapterIndexes(_ index: String) {
        #expect(throws: ClientError.self) { try source("[{\"index\":\(index),\"title\":\"Opening\",\"start\":0,\"end\":60}]") }
    }

    @Test func rejectsDuplicateIndexesUnknownFieldsAndOversizedLists() {
        #expect(throws: ClientError.self) { try source("[{\"index\":0,\"start\":0,\"end\":30},{\"index\":0,\"start\":30,\"end\":60}]") }
        #expect(throws: ClientError.self) { try source("[{\"index\":0,\"start\":0,\"end\":60,\"unknown\":true}]") }
        #expect(throws: ClientError.self) { try source("[" + Array(repeating: "{}", count: 1025).joined(separator: ",") + "]") }
    }

    @Test func expiredPairingHasConnectionRecovery() async throws {
        let fixture = try HTTPFixture(body: "{\"error\":\"not found\"}", status: 404)
        defer { fixture.remove() }
        await #expect(throws: ClientError.invalidInput("That code expired or was cancelled. Connect again to get a new code.")) {
            try await fixture.client.pollQuickConnect(secret: "expired-secret")
        }
        #expect(fixture.requests.count == 1)
    }
}
