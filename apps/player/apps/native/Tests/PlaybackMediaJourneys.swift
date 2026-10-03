import AVFoundation
import Foundation
import Testing
@testable import KinosailPlayer

// Opt-in decoded-media E2E. Generate the bounded fictional sample using the
// command in the failure-analysis note, then run this suite with StartupJourneys.
@Suite(.serialized, .enabled(if: FileManager.default.fileExists(atPath: "/tmp/kinosail-appletv-task7-media.mp4")))
@MainActor struct PlaybackMediaJourneys {
    @Test func coldAndPreparedPlaybackDecodeResumeSeekPauseAndRepeat() async throws {
        let data = try Data(contentsOf: URL(fileURLWithPath: "/tmp/kinosail-appletv-task7-media.mp4"))
        try #require(data.count <= 1024 * 1024)
        let fixture = try await PlaybackStartupFixture()
        fixture.denied = false; fixture.media = data
        defer { fixture.close() }
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: folder) }
        let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
        for prepared in [false, true] {
            let engine = PlaybackEngine()
            defer { engine.stop() }
            let item = try fixture.item("movie")
            if prepared { try await engine.prepare(item, client: fixture.client) }
            let start = ContinuousClock.now
            try await engine.play(item, client: fixture.client, store: store)
            let player = try #require(engine.player)
            let playerItem = try #require(player.currentItem)
            let output = AVPlayerItemVideoOutput(pixelBufferAttributes: [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA])
            playerItem.add(output)
            let initial = player.currentTime().seconds
            var decoded = false
            for _ in 0..<500 {
                if player.currentTime().seconds > initial + 0.05,
                   output.copyPixelBuffer(forItemTime: player.currentTime(), itemTimeForDisplay: nil) != nil { decoded = true; break }
                try await Task.sleep(for: .milliseconds(10))
            }
            let elapsed = start.duration(to: .now)
            #expect(decoded)
            #expect(playerItem.status == .readyToPlay)
            #expect(abs(initial - 3) < 0.5)
            #expect(!engine.usingCompatibility)
            try await engine.seek(to: 4)
            engine.pause()
            #expect(abs(player.currentTime().seconds - 4) < 0.1)
            #expect(player.timeControlStatus == .paused)
            engine.resume()
            for _ in 0..<300 where player.currentTime().seconds <= 4.1 { try await Task.sleep(for: .milliseconds(10)) }
            #expect(player.currentTime().seconds > 4.1)
            print("MEDIA prepared=\(prepared) firstDecodedMovingFrame=\(elapsed) resume=\(initial) seekPauseResume=true")
            engine.stop()
            #expect(engine.player == nil)
        }
        await fixture.client.close(purgeCache: true)
    }
}
