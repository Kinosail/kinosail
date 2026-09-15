import Foundation
import Testing
@testable import KinosailPlayer

struct MediaOperationTests {
    @Test func mapsShortenedPlaybackExactlyOnce() throws {
        let timeline = try MediaTimeline(sourceDuration: 100, duration: 70, omitted: [0..<10, 40..<60])
        #expect(timeline.sourceTime(0) == 10)
        #expect(timeline.sourceTime(30) == 60)
        #expect(timeline.sourceTime(70) == 100)
        #expect(timeline.presentationTime(5) == 0)
        #expect(timeline.presentationTime(50) == 30)
        #expect(timeline.presentationTime(80) == 50)
        #expect(try MediaTimeline(timeline.json) == timeline)
    }

    @Test func rejectsInvalidTimelineAndCurrentServerProgressFields() throws {
        #expect(throws: ClientError.self) { try MediaTimeline(sourceDuration: 100, duration: 90, omitted: [0..<10, 5..<15]) }
        #expect(throws: ClientError.self) { try MediaTimeline(sourceDuration: 100, duration: 90, omitted: [100..<110]) }
        #expect(throws: ClientError.self) { try MediaTimeline(sourceDuration: 100, duration: 90) }
        #expect(throws: ClientError.self) { try MediaTimeline(sourceDuration: .infinity, duration: 90) }
        let state = try WatchProgress(StrictJSON.decode(Data("{\"seconds\":10,\"updated\":\"2026-09-10T13:00:00Z\",\"dismissed\":false}".utf8)))
        #expect(state.seconds == 10)
        #expect(throws: ClientError.self) { try WatchProgress(.object(["readerPage": .number(0), "readerOffset": .number(0.5)])) }
    }

    @Test func validatesPreferencesAndBookmarksBeforeSideEffects() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        var playback = PlaybackPreferences()
        playback.rate = .nan
        await #expect(throws: ClientError.self) { try await fixture.client.savePlaybackPreferences(itemID: "movie", preferences: playback) }
        var media = MediaPreferences()
        media.readerFontSize = 33
        await #expect(throws: ClientError.self) { try await fixture.client.saveMediaPreferences(media) }
        media = MediaPreferences(); media.downloadLimitGiB = -1
        await #expect(throws: ClientError.self) { try await fixture.client.saveMediaPreferences(media) }
        await #expect(throws: ClientError.self) { try await fixture.client.addBookmark(itemID: "movie", title: "", position: .playback(seconds: 1)) }
        await #expect(throws: ClientError.self) { try await fixture.client.addBookmark(itemID: "book", title: "Page", position: .reading(page: 1, offset: 1.01)) }
        await #expect(throws: ClientError.self) { try await fixture.client.addBookmark(itemID: "movie", title: "Scene", position: .playback(seconds: -.infinity)) }
        await #expect(throws: ClientError.self) { try await fixture.client.removeBookmark(itemID: "movie", bookmarkID: "../bookmark") }
        await #expect(throws: ClientError.self) { try await fixture.client.syncProgress(itemID: "movie", progress: WatchProgress(), expected: WatchProgress(), playbackToken: "") }
        #expect(fixture.requests.isEmpty)
    }

    @Test(arguments: [
        "{\"id\":\"ID\",\"title\":\"Scene\",\"seconds\":1,\"page\":1}",
        "{\"id\":\"ID\",\"title\":\"Scene\",\"seconds\":1,\"offset\":0}",
        "{\"id\":\"ID\",\"title\":\"Scene\",\"page\":0}",
        "{\"id\":\"ID\",\"title\":\"Scene\",\"seconds\":true}",
        "{\"id\":\"ID\",\"title\":\"Scene\",\"seconds\":1,\"extra\":null}"
    ])
    func rejectsConflictingAndMalformedBookmarks(_ text: String) {
        let raw = text.replacingOccurrences(of: "ID", with: String(repeating: "a", count: 64))
        #expect(throws: ClientError.self) { try Bookmark(StrictJSON.decode(Data(raw.utf8))) }
    }
}
