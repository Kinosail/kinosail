#if os(tvOS)
import AVFoundation
import CoreMedia
import Testing
@testable import KinosailPlayer

@MainActor
private final class FakeDisplay: DisplayModeSwitching {
    var isDisplayCriteriaMatchingEnabled: Bool
    var isDisplayModeSwitchInProgress = false
    var preferredDisplayCriteria: AVDisplayCriteria? {
        didSet {
            assignments.append(preferredDisplayCriteria)
            if preferredDisplayCriteria != nil, switchLasts > .zero {
                isDisplayModeSwitchInProgress = true
                let duration = switchLasts
                Task { @MainActor in
                    try? await Task.sleep(for: duration)
                    self.isDisplayModeSwitchInProgress = false
                }
            }
        }
    }
    var assignments: [AVDisplayCriteria?] = []
    var switchLasts: Duration

    init(enabled: Bool, switchLasts: Duration = .zero) {
        isDisplayCriteriaMatchingEnabled = enabled
        self.switchLasts = switchLasts
    }
}

private func criteria(fps: Float) throws -> AVDisplayCriteria {
    var format: CMVideoFormatDescription?
    let status = CMVideoFormatDescriptionCreate(allocator: nil, codecType: kCMVideoCodecType_H264, width: 1920, height: 1080, extensions: nil, formatDescriptionOut: &format)
    #expect(status == noErr)
    return AVDisplayCriteria(refreshRate: fps, formatDescription: try #require(format))
}

@MainActor
struct DisplayMatchingTests {
    @Test func appliesTheVideoCriteriaAndWaitsForTheTelevisionToSwitch() async throws {
        let display = FakeDisplay(enabled: true, switchLasts: .milliseconds(400))
        let clock = ContinuousClock()
        let started = clock.now
        let outcome = await DisplayMatching.match(try criteria(fps: 23.976), on: display)
        #expect(outcome == .applied)
        #expect(display.assignments.count == 1)
        #expect(display.preferredDisplayCriteria != nil)
        #expect(!display.isDisplayModeSwitchInProgress)
        #expect(started.duration(to: clock.now) >= .milliseconds(400))
    }

    @Test func leavesTheDisplayAloneWhenMatchContentIsOff() async throws {
        let display = FakeDisplay(enabled: false)
        #expect(await DisplayMatching.match(try criteria(fps: 24), on: display) == .disabled)
        #expect(display.assignments.isEmpty)
    }

    @Test func leavesTheDisplayAloneWithoutCriteriaOrWindow() async {
        let display = FakeDisplay(enabled: true)
        #expect(await DisplayMatching.match(nil, on: display) == .unavailable)
        #expect(await DisplayMatching.match(nil, on: nil) == .unavailable)
        #expect(display.assignments.isEmpty)
    }

    @Test func startsPlaybackAnywayWhenTheSwitchNeverFinishes() async throws {
        let display = FakeDisplay(enabled: true, switchLasts: .seconds(60))
        let clock = ContinuousClock()
        let started = clock.now
        let outcome = await DisplayMatching.match(try criteria(fps: 50), on: display, timeout: .milliseconds(300))
        #expect(outcome == .timedOut)
        #expect(started.duration(to: clock.now) < .seconds(2))
    }

    @Test func resetReturnsTheDisplayToItsDefaultMode() async throws {
        let display = FakeDisplay(enabled: true)
        _ = await DisplayMatching.match(try criteria(fps: 25), on: display)
        DisplayMatching.reset(display)
        #expect(display.preferredDisplayCriteria == nil)
        #expect(display.assignments.count == 2)
    }
}
#endif
