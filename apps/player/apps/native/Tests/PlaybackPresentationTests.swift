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

    @Test func invalidNativePlayerTimesCannotChangeProgress() {
        let engine = PlaybackEngine()
        engine.seconds = 7
        engine.lastNowPlayingSecond = 7
        for time in [CMTime.indefinite, CMTime(value: -1, timescale: 1),
                     CMTime(seconds: 31_536_001, preferredTimescale: 600), CMTime(value: Int64.max, timescale: 1)] {
            engine.tick(time: time, attempt: engine.generation)
            #expect(engine.seconds == 7)
            #expect(engine.lastNowPlayingSecond == 7)
        }
    }

    #if os(tvOS)
    @Test func televisionWaitsForVideoAndClearsPresentation() throws {
        let presentation = PlayerPresentation()
        let actions = presentation.controller.transportBarCustomMenuItems.compactMap { $0 as? UIAction }
        let options = try #require(actions.first { $0.title == "Playback options" })
        let seekPreview = try #require(actions.first { $0.title == "Seek with preview" })
        #expect(!presentation.readyForDisplay)
        presentation.controller.player = AVPlayer()
        var opened = 0
        let control = UIButton(type: .system)
        control.addAction(options, for: .primaryActionTriggered)
        presentation.showOptions = { opened += 1 }
        control.sendActions(for: .primaryActionTriggered)
        #expect(opened == 1)
        presentation.showOptions = { opened += 10 }
        control.sendActions(for: .primaryActionTriggered)
        #expect(opened == 11)
        var previewOpened = 0
        let previewControl = UIButton(type: .system)
        previewControl.addAction(seekPreview, for: .primaryActionTriggered)
        presentation.showSeekPreview = { previewOpened += 1 }
        previewControl.sendActions(for: .primaryActionTriggered)
        #expect(previewOpened == 1)
        presentation.showCaptions("A caption")
        #expect(presentation.controller.showsPlaybackControls)
        #expect(presentation.controller.transportBarCustomMenuItems.count == 2)
        #expect(presentation.controller.transportBarCustomMenuItems.contains { $0 === options })
        #expect(presentation.controller.transportBarCustomMenuItems.contains { $0 === seekPreview })
        #expect(!presentation.readyForDisplay)
        presentation.clear()
        #expect(presentation.controller.player == nil)
        #expect(presentation.controller.transportBarCustomMenuItems.contains { $0 === options })
        #expect(presentation.controller.transportBarCustomMenuItems.contains { $0 === seekPreview })
        #expect(presentation.showOptions == nil)
        #expect(presentation.showSeekPreview == nil)
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
