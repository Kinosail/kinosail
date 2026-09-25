import Foundation
import Testing
#if os(iOS) || os(tvOS)
import UIKit
#endif
@testable import KinosailPlayer

struct PlaybackChapterTests {
    private func source(_ chapters: String, start: String = "0", preview: String = "", token: String = "") throws -> PlaybackSource {
        let body = """
        {"media":{"duration":60},"plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"duration":60,"start":\(start),"directAllowed":true,"direct":"/media/movie","directType":"video/mp4","chapters":\(chapters),"trickplay":"\(preview)","progressToken":"\(token)"}
        """
        return try PlaybackSource(StrictJSON.decode(Data(body.utf8)), itemID: "movie", server: ServerAddress("https://example.com"))
    }

    private func sourceWithDirectType(_ directType: String) throws -> PlaybackSource {
        let escapedDirectType = directType.replacingOccurrences(of: "\"", with: "\\\"")
        let body = """
        {"media":{"duration":60},"plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"duration":60,"start":0,"directAllowed":true,"direct":"/media/movie","directType":"\(escapedDirectType)","compatibleDuration":60,"compatible":"/hls/movie/index.m3u8","compatiblePlan":{"allowed":true,"mode":"remux","reason":"compatibility-requested"},"chapters":[]}
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

    @Test func prefersCompatibleSourceForUnsupportedAppleTVOriginals() throws {
        #if os(tvOS)
        let unsupportedOriginal = true
        #else
        let unsupportedOriginal = false
        #endif
        #expect(try sourceWithDirectType("video/mp4; codecs=\"avc1.640028, mp4a.40.2\"").shouldPreferCompatibleOnAppleTV == false)
        #expect(try sourceWithDirectType("video/x-matroska").shouldPreferCompatibleOnAppleTV == unsupportedOriginal)
        #expect(try sourceWithDirectType("").shouldPreferCompatibleOnAppleTV == unsupportedOriginal)
    }

    @Test func validatesTrickplayTemplateBeforeUse() throws {
        #expect(try source("[]", preview: "/trickplay/movie/{second}").trickplay == "/trickplay/movie/{second}")
        #expect(try source("[]", preview: "/trickplay/movie/{second}?playbackToken=valid", token: "valid").trickplay != nil)
        for invalid in ["/trickplay/other/{second}", "/trickplay/movie/20", "/trickplay/movie/{second}?unknown=1",
                        "https://other.example/trickplay/movie/{second}", "/trickplay/movie/{second}#fragment"] {
            #expect(throws: ClientError.self) { try source("[]", preview: invalid) }
        }
        for invalid in ["/trickplay/movie/{second}?playbackToken=wrong", "/trickplay/movie/{second}?playbackToken=valid&playbackToken=valid",
                        "/trickplay/movie/{second}?playbackToken=valid#fragment"] {
            #expect(throws: ClientError.self) { try source("[]", preview: invalid, token: "valid") }
        }
        #expect(throws: ClientError.self) { try source("[]", preview: String(repeating: "x", count: 16_385)) }
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

#if os(iOS) || os(tvOS)
struct PlaybackFramePreviewTests {
    @Test func loadsAuthenticatedFrame() async throws {
        let format = UIGraphicsImageRendererFormat()
        format.scale = 1
        let data = UIGraphicsImageRenderer(size: CGSize(width: 4, height: 4), format: format).jpegData(withCompressionQuality: 0.8) { context in
            context.cgContext.setFillColor(UIColor.red.cgColor)
            context.cgContext.fill(CGRect(x: 0, y: 0, width: 4, height: 4))
        }
        let fixture = try HTTPFixture(data: data, headers: ["Content-Type": "image/jpeg"])
        defer { fixture.remove() }
        let image = try await PlaybackFramePreview.load(template: "/trickplay/movie/{second}", client: fixture.client, asset: nil, second: 20)
        #expect(image.size == CGSize(width: 4, height: 4))
        #expect(fixture.requests.map { $0.url?.path } == ["/trickplay/movie/20"])
        #expect(fixture.requests.first?.value(forHTTPHeaderField: "Authorization") == "Bearer fixture-token")
    }

    @Test func rejectsInvalidFrameTimesWithoutRequest() async throws {
        let fixture = try HTTPFixture(body: "not an image", headers: ["Content-Type": "image/jpeg"])
        defer { fixture.remove() }
        for second in [-10, 1, 43210] {
            await #expect(throws: ClientError.self) {
                try await PlaybackFramePreview.load(template: "/trickplay/movie/{second}", client: fixture.client, asset: nil, second: second)
            }
        }
        #expect(fixture.requests.isEmpty)
    }

    @Test func rejectsInvalidFrameData() async throws {
        let fixture = try HTTPFixture(body: "not an image", headers: ["Content-Type": "image/jpeg"])
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) {
            try await PlaybackFramePreview.load(template: "/trickplay/movie/{second}", client: fixture.client, asset: nil, second: 20)
        }
        #expect(fixture.requests.map { $0.url?.path } == ["/trickplay/movie/20"])
    }
}
#endif
