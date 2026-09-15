import AVKit
import Testing
@testable import KinosailPlayer

@MainActor struct PlaybackPresentationTests {
    @Test func pictureInPictureRestorationWaitsForVisibleSurface() {
        let presentation = PlayerPresentation()
        var requested = 0
        var completions: [Bool] = []
        presentation.restore = { requested += 1 }
        presentation.restoreInterface { completions.append($0) }
        #expect(requested == 1)
        #expect(completions.isEmpty)
        presentation.appeared()
        presentation.appeared()
        #expect(completions == [true])
    }

    @Test func unavailableAndCancelledRestorationsCompleteExactlyOnce() {
        let presentation = PlayerPresentation()
        var completions: [Bool] = []
        presentation.restoreInterface { completions.append($0) }
        #expect(completions == [false])
        presentation.restore = {}
        presentation.restoreInterface { completions.append($0) }
        presentation.clear()
        presentation.appeared()
        #expect(completions == [false, false])
    }

    @Test func replacingRestorationResolvesTheOldRequest() {
        let presentation = PlayerPresentation()
        presentation.restore = {}
        var old: [Bool] = [], current: [Bool] = []
        presentation.restoreInterface { old.append($0) }
        presentation.restoreInterface { current.append($0) }
        #expect(old == [false])
        #expect(current.isEmpty)
        presentation.appeared()
        #expect(current == [true])
    }

    @Test func alreadyVisibleRestorationDoesNotOpenAnotherPlayer() {
        let presentation = PlayerPresentation()
        var opens = 0
        presentation.restore = { opens += 1 }
        presentation.appeared()
        var restored = false
        presentation.restoreInterface { restored = $0 }
        #expect(restored)
        #expect(opens == 0)
    }

    @Test(arguments: [Double.nan, .infinity, -.infinity, -1, 0, 0.49, 3.01])
    func invalidSpeedCannotChangeSessionState(_ rate: Double) async {
        let playback = PlaybackCoordinator()
        await #expect(throws: ClientError.self) { try await playback.changeRate(rate) }
        #expect(playback.playbackRate == 1)
        #expect(playback.player == nil)
        #expect(playback.selectedSubtitleTrackID == nil)
    }

    #if os(tvOS)
    @Test func televisionWaitsForVideoAndClearsPresentation() {
        let presentation = PlayerPresentation()
        #expect(!presentation.readyForDisplay)
        presentation.controller.player = AVPlayer()
        presentation.showCaptions("A caption")
        #expect(presentation.controller.showsPlaybackControls)
        #expect(!presentation.readyForDisplay)
        presentation.clear()
        #expect(presentation.controller.player == nil)
        #expect(presentation.captionText.isEmpty)
        #expect(!presentation.readyForDisplay)
    }
    #endif

    #if os(iOS)
    @Test func clearingReleasesVideoAndCaptionStateWithoutCropping() {
        let presentation = PlayerPresentation()
        presentation.attach(AVPlayer())
        presentation.showCaptions("A caption")
        #expect(presentation.videoView.playerLayer.player != nil)
        #expect(presentation.videoView.playerLayer.videoGravity == .resizeAspect)
        presentation.clear()
        #expect(presentation.videoView.playerLayer.player == nil)
        #expect(presentation.captionText.isEmpty)
        #expect(!presentation.readyForDisplay)
        #expect(!presentation.pictureInPicturePossible)
    }

    @Test(.enabled(if: AVPictureInPictureController.isPictureInPictureSupported()))
    func obsoletePictureInPictureCallbacksCannotCloseNewPlayback() throws {
        let presentation = PlayerPresentation()
        let obsolete = try #require(AVPictureInPictureController(playerLayer: AVPlayerLayer(player: AVPlayer())))
        var closed = 0
        presentation.closedPictureInPicture = { closed += 1 }
        presentation.attach(AVPlayer())
        presentation.pictureInPictureControllerWillStartPictureInPicture(obsolete)
        presentation.pictureInPictureControllerDidStopPictureInPicture(obsolete)
        var restored = true
        presentation.pictureInPictureController(obsolete, restoreUserInterfaceForPictureInPictureStopWithCompletionHandler: { restored = $0 })
        #expect(closed == 0)
        #expect(!presentation.pictureInPicture)
        #expect(!restored)
        #expect(presentation.videoView.playerLayer.player != nil)
    }
    #endif
}
