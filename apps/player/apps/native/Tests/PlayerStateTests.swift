import AVFoundation
import Foundation
import Testing
@testable import KinosailPlayer

struct PlayerStateTests {
    @Test func rewritesAllHLSResourcesToLocalCapabilities() throws {
        let server = try ServerAddress("https://media.example")
        let source = try server.mediaURL("/hls/movie/p/recipe/index.m3u8")
        let data = Data("""
        #EXTM3U
        #EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",URI="audio.m3u8"
        #EXT-X-MAP:URI="init.mp4"
        #EXTINF:6,
        segment.ts
        #EXT-X-ENDLIST
        """.utf8)
        var resources: [URL] = []
        let result = try HLSManifest.rewrite(data, source: source, server: server, itemID: "movie") { url in
            resources.append(url)
            return URL(string: "http://127.0.0.1:1234/private/\(resources.count)")!
        }
        let text = String(decoding: result, as: UTF8.self)
        #expect(resources.map(\.lastPathComponent) == ["audio.m3u8", "init.mp4", "segment.ts"])
        #expect(!text.contains("media.example"))
        #expect(text.contains("URI=\"http://127.0.0.1:1234/private/1\""))
    }

    @Test(arguments: [
        "#EXTM3U\nhttps://other.example/hls/movie/segment.ts\n",
        "#EXTM3U\nsegment.ts\nhttps://other.example/second.ts\n",
        "#EXTM3U\n/api/v1/me\n", "#EXTM3U\n/hls/another/segment.ts\n",
        "#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=https://other.example/key\n",
        "#EXTM3U\n#EXT-X-MAP:URI=\"//other.example/key\"\n",
        "#EXTM3U\n#EXT-X-DEFINE:NAME=\"path\",VALUE=\"secret\"\n",
        "#EXTM3U\n{$path}\n", "not a playlist\n"
    ])
    func rejectsPlaylistEscapeBeforeRegisteringAnyResource(_ text: String) throws {
        let server = try ServerAddress("https://media.example")
        var calls = 0
        #expect(throws: ClientError.self) {
            try HLSManifest.rewrite(Data(text.utf8), source: server.mediaURL("/hls/movie/index.m3u8"), server: server, itemID: "movie") { url in calls += 1; return url }
        }
        #expect(calls == 0)
    }

    @Test(arguments: ["bytes=0-0", "bytes=100-", "bytes=-100", "bytes=2-20"])
    func acceptsOneBoundedRange(_ raw: String) throws { #expect(try MediaRange.validate(raw) == raw) }

    @Test(arguments: ["bytes=-0", "bytes=10-9", "bytes=1-2,3-4", "bytes=+1-2", "bytes=0-1\r\nX: y", "bytes=99999999999999999999-", "bytes=-", "0-10"])
    func rejectsMalformedRanges(_ raw: String) { #expect(throws: ClientError.self) { try MediaRange.validate(raw) } }

    @Test @MainActor func shuffleRetainsCurrentTrackAndRestoresAlbumOrder() throws {
        let server = try ServerAddress("https://media.example")
        let tracks = try ["one", "two", "three"].map { id in try MediaItem(.object(["id": .string(id), "kind": .string("audio"), "title": .string(id)]), server: server) }
        let queue = MediaQueue()
        try queue.replace(tracks, startingAt: 1)
        queue.setShuffle(true)
        #expect(queue.items[queue.currentIndex!].id == "two")
        #expect(Set(queue.items.map(\.id)) == Set(tracks.map(\.id)))
        queue.setShuffle(false)
        #expect(queue.items.map(\.id) == ["one", "two", "three"])
        #expect(queue.currentIndex == 1)
        queue.setRepeat(.one)
        #expect(queue.next(automatic: true)?.id == "two")
        #expect(queue.next()?.id == "three")
        #expect(queue.next() == nil)
        queue.setRepeat(.all)
        #expect(queue.next()?.id == "one")
        #expect(throws: ClientError.self) { try queue.replace([tracks[0], tracks[0]], startingAt: 0) }
        #expect(queue.currentIndex == 0)
    }

    @Test @MainActor func networkRecoveryRejectsDenialsAndFormatFailures() {
        for code in [408, 429, 500, 502, 503, 504] { #expect(PlaybackCoordinator.isNetworkFailure(ClientError.http(code))) }
        for code in [400, 401, 403, 404, 410, 413] { #expect(!PlaybackCoordinator.isNetworkFailure(ClientError.http(code))) }
        for code: URLError.Code in [.timedOut, .networkConnectionLost, .notConnectedToInternet] {
            #expect(PlaybackCoordinator.isNetworkFailure(URLError(code)))
            #expect(PlaybackCoordinator.isNetworkFailure(NSError(domain: AVFoundationErrorDomain, code: AVError.failedToLoadMediaData.rawValue, userInfo: [NSUnderlyingErrorKey: URLError(code)])))
        }
        for error: Error in [URLError(.cancelled), URLError(.serverCertificateUntrusted), ClientError.invalidResponse, NSError(domain: AVFoundationErrorDomain, code: AVError.decoderNotFound.rawValue)] {
            #expect(!PlaybackCoordinator.isNetworkFailure(error))
        }
    }

    @Test @MainActor func compatibilityRequiresDecoderEvidence() {
        #expect(PlaybackCoordinator.isFormatFailure(NSError(domain: AVFoundationErrorDomain, code: AVError.decoderNotFound.rawValue)))
        #expect(!PlaybackCoordinator.isFormatFailure(URLError(.timedOut)))
        #expect(!PlaybackCoordinator.isFormatFailure(NSError(domain: AVFoundationErrorDomain, code: AVError.contentIsNotAuthorized.rawValue)))
        let loadFailure = AVError.failedToLoadMediaData.rawValue
        #expect(PlaybackCoordinator.isFormatFailure(NSError(domain: AVFoundationErrorDomain, code: loadFailure,
                                                           userInfo: [NSUnderlyingErrorKey: NSError(domain: NSOSStatusErrorDomain, code: -12873)])))
        for cause in [URLError(.timedOut) as NSError, NSError(domain: NSOSStatusErrorDomain, code: -1)] {
            #expect(!PlaybackCoordinator.isFormatFailure(NSError(domain: AVFoundationErrorDomain, code: loadFailure, userInfo: [NSUnderlyingErrorKey: cause])))
        }
        #expect(!PlaybackCoordinator.isFormatFailure(NSError(domain: AVFoundationErrorDomain, code: loadFailure)))
    }

    @Test func persistsCanonicalProgressAndRejectsCrossProfileSyncBeforeNetwork() async throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: folder) }
        let scope = String(repeating: "a", count: 64)
        let store = try ProgressSyncStore(scope: scope, directory: folder)
        var progress = WatchProgress(); progress.seconds = 120; progress.session = "playback-one"; progress.revision = 1
        await #expect(throws: ClientError.self) { try await store.record(itemID: "../movie", progress: progress, expected: WatchProgress()) }
        #expect(!FileManager.default.fileExists(atPath: folder.path))
        try await store.record(itemID: "movie", progress: progress, expected: WatchProgress())
        progress.revision = 2; progress.seconds = 140
        try await store.record(itemID: "movie", progress: progress, expected: progress)
        let restored = try ProgressSyncStore(scope: scope, directory: folder)
        let entries = try await restored.pending()
        #expect(entries.count == 1)
        #expect(entries[0].progress.seconds == 140)
        #expect(entries[0].expected == WatchProgress())
        let data = try Data(contentsOf: folder.appendingPathComponent(scope + ".json"))
        #expect(!String(decoding: data, as: UTF8.self).contains("playbackToken"))
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await restored.synchronize(client: fixture.client) }
        #expect(fixture.requests.isEmpty)
        #expect(try await restored.pending().count == 1)
    }
}
