#if os(tvOS)
import AVFoundation
import Foundation
import Testing
import UIKit
@testable import KinosailPlayer

@Suite(.serialized, .enabled(if: FileManager.default.fileExists(atPath: "/tmp/kinosail-player-buffer-bar.mp4")))
@MainActor struct NowPlayingMetadataJourneys {
    @Test func playingEpisodePublishesTitleEpisodeDescriptionAndArtworkThroughAVKit() async throws {
        let media = try Data(contentsOf: URL(fileURLWithPath: "/tmp/kinosail-player-buffer-bar.mp4"))
        let fixture = try await PlaybackStartupFixture()
        fixture.denied = false; fixture.media = media
        let poster = try #require(UIGraphicsImageRenderer(size: CGSize(width: 64, height: 96)).image { context in
            UIColor.systemTeal.setFill(); context.fill(CGRect(x: 0, y: 0, width: 64, height: 96))
        }.pngData())
        fixture.images = ["/episode-art/movie": poster]
        defer { fixture.close() }
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: folder) }
        let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
        let item = try MediaItem(.object(["id": .string("movie"), "kind": .string("video"), "title": .string("The Long Way Home"),
                                          "show": .string("Into the Wild"), "showId": .string("0123456789abcdef"), "season": .number(1),
                                          "episode": .number(2), "plot": .string("A fictional journey home."),
                                          "artwork": .string("/episode-art/movie")]), server: fixture.server)
        let coordinator = PlaybackCoordinator()
        defer { coordinator.stop() }
        try await coordinator.play(item, client: fixture.client, store: store)
        let nativeItem = try #require(coordinator.player?.currentItem)
        var artwork: Data?
        let deadline = ContinuousClock.now.advanced(by: .seconds(5))
        while artwork == nil, ContinuousClock.now < deadline {
            artwork = nativeItem.externalMetadata.first { $0.identifier == .commonIdentifierArtwork }?.dataValue
            if artwork == nil { try await Task.sleep(for: .milliseconds(20)) }
        }
        func text(_ identifier: AVMetadataIdentifier) -> String? {
            nativeItem.externalMetadata.first { $0.identifier == identifier }?.stringValue
        }
        print("NOW_PLAYING title=\(text(.commonIdentifierTitle) ?? "nil") subtitle=\(text(.iTunesMetadataTrackSubTitle) ?? "nil") description=\(text(.commonIdentifierDescription) != nil) artworkBytes=\(artwork?.count ?? 0)")
        #expect(text(.commonIdentifierTitle) == "The Long Way Home")
        #expect(text(.iTunesMetadataTrackSubTitle) == "Into the Wild · S1, E2")
        #expect(text(.commonIdentifierDescription) == "A fictional journey home.")
        #expect((artwork?.count ?? 0) > 0)
        #expect(artwork.flatMap(UIImage.init(data:)) != nil)
        await fixture.client.close(purgeCache: true)
    }
}
#endif
