#if os(tvOS)
import Testing
import UIKit
@testable import KinosailPlayer

@MainActor struct RemoteTouchDelayTests {
    @Test func everyRemoteTouchPassesThroughTheGuardedApplication() {
        #expect(RemoteTouchDelays.guardsEvents)
        RemoteTouchDelays.install()
        #expect(RemoteTouchDelays.guardsEvents, "Installing twice must not swap the original event delivery back in")
    }

    @Test func recognizersAddedDuringPlaybackStopDelayingBeforeTheNextTouch() {
        let first = UIView(frame: CGRect(x: 0, y: 0, width: 1920, height: 1080))
        let second = UIView(frame: first.frame)
        let nested = UIView(), deeper = UIView()
        first.addSubview(nested); nested.addSubview(deeper)
        let onWindow = UITapGestureRecognizer(), onNested = UIPanGestureRecognizer(), onDeeper = UISwipeGestureRecognizer(), onSecond = UILongPressGestureRecognizer()
        onNested.delaysTouchesBegan = true
        first.addGestureRecognizer(onWindow); nested.addGestureRecognizer(onNested); deeper.addGestureRecognizer(onDeeper); second.addGestureRecognizer(onSecond)
        #expect([onWindow, onNested, onDeeper, onSecond].allSatisfy { $0.delaysTouchesEnded })
        RemoteTouchDelays.release(in: [first, second])
        for recognizer in [onWindow, onNested, onDeeper, onSecond] {
            #expect(!recognizer.delaysTouchesBegan)
            #expect(!recognizer.delaysTouchesEnded)
        }
        let late = UIPanGestureRecognizer()
        deeper.addGestureRecognizer(late)
        RemoteTouchDelays.release(in: [first, second])
        #expect(!late.delaysTouchesEnded)
        RemoteTouchDelays.release(in: [first, second])
        #expect([onWindow, onNested, onDeeper, onSecond, late].allSatisfy { !$0.delaysTouchesBegan && !$0.delaysTouchesEnded })
    }

    @Test(arguments: [
        ([UITouch.Phase.began], true),
        ([.moved], false), ([.stationary], false), ([.ended], false), ([.cancelled], false),
        ([.began, .moved], false), ([], false),
    ])
    func flagsChangeOnlyBeforeAGestureStarts(_ phases: [UITouch.Phase], _ releases: Bool) {
        #expect(RemoteTouchDelays.startsGesture(phases) == releases)
    }
}
#endif
