import Foundation
import Testing
import AVFoundation
@testable import KinosailPlayer

// Failure cases: unknown duration, nonfinite/reversed/outside ranges, too many
// ranges, and omitted source sections must never paint fabricated buffer data.
struct BufferedPlaybackTimelineTests {
    @Test func loadedRangesPreserveGapsAndClipAtMediaBounds() throws {
        let timeline = try MediaTimeline(sourceDuration: 100, duration: 100)
        #expect(timeline.bufferedRanges(for: [(-20, 20), (40, 60), (90, 200)]) == [0..<20, 40..<60, 90..<100])
        #expect(timeline.bufferedRanges(for: [(30, 20), (110, 120), (.nan, 40), (40, .infinity), (-20, -10)]).isEmpty)
        #expect(timeline.bufferedRanges(for: []).isEmpty)
    }

    @Test func compatibilityBuffersMapAroundOmittedSourceSections() throws {
        let timeline = try MediaTimeline(sourceDuration: 100, duration: 70, omitted: [20..<40, 70..<80])
        #expect(timeline.bufferedRanges(for: [(10, 60)]) == [10..<20, 40..<70, 80..<90])
        #expect(timeline.bufferedRanges(for: [(20, 30)]) == [40..<50])
        #expect(timeline.bufferedRanges(for: [(0, 70)]) == [0..<20, 40..<70, 80..<100])
    }

    @Test func missingDurationAndExcessiveRangesStayEmpty() throws {
        let unknown = try MediaTimeline(sourceDuration: 0, duration: 0)
        #expect(unknown.bufferedRanges(for: [(0, 60)]).isEmpty)
        let known = try MediaTimeline(sourceDuration: 100, duration: 100)
        #expect(known.bufferedRanges(for: Array(repeating: (0, 60), count: 129)).isEmpty)
    }
}

@MainActor struct BufferedPlaybackStateTests {
    // Missing, pending, and stopped playback must clear a previous item's buffer.
    @Test func missingAndPendingMediaClearStaleRangesWithoutSeeking() throws {
        let engine = PlaybackEngine()
        engine.bufferedRanges = [0..<60]
        engine.refreshBufferedRanges()
        #expect(engine.bufferedRanges.isEmpty)
        engine.timeline = try MediaTimeline(sourceDuration: 100, duration: 100)
        engine.player = AVPlayer(playerItem: AVPlayerItem(url: URL(string: "https://media.example.invalid/pending.mp4")!))
        engine.bufferedRanges = [0..<60]
        engine.refreshBufferedRanges()
        #expect(engine.bufferedRanges.isEmpty)
        #expect(engine.seconds == 0)
        #expect(engine.player?.rate == 0)
        engine.bufferedRanges = [0..<60]
        engine.stop()
        #expect(engine.bufferedRanges.isEmpty)
    }
}

#if os(iOS)
import UIKit
import SwiftUI

@Suite(.serialized) @MainActor
struct BufferedPlaybackSliderTests {
    // Native playback journeys do not assert UIKit target/action ordering. These
    // checks protect seek commits if actor isolation introduces deferred callbacks.
    @Test func nontrackingValueChangeFinishesEditingSynchronouslyAfterUpdatingTheBinding() async throws {
        var position = 20.0
        var events: [String] = []
        let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: BufferedPlaybackSlider(
            value: Binding(get: { position }, set: { position = $0; events.append("value:\($0)") }),
            duration: 100, buffered: [],
            onEditingChanged: { events.append("\($0 ? "begin" : "end"):\(position)") }))
        window.makeKeyAndVisible()
        defer { window.isHidden = true; previous?.makeKey() }
        window.layoutIfNeeded()
        try await Task.sleep(for: .milliseconds(100))
        let slider = try #require(findSlider(window))
        #expect(!slider.isTracking)
        slider.value = 35
        slider.sendActions(for: .valueChanged)
        #expect(position == 35)
        #expect(events == ["begin:20.0", "value:35.0", "end:35.0"])
    }

    @Test(arguments: [UIControl.Event.touchUpInside.rawValue, UIControl.Event.touchUpOutside.rawValue,
                      UIControl.Event.touchCancel.rawValue])
    func touchCompletionAndCancellationEndEditingSynchronouslyWithoutChangingPosition(_ event: UInt) async throws {
        var position = 20.0
        var editing: [Bool] = []
        let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: BufferedPlaybackSlider(
            value: Binding(get: { position }, set: { position = $0 }), duration: 100, buffered: [],
            onEditingChanged: { editing.append($0) }))
        window.makeKeyAndVisible()
        defer { window.isHidden = true; previous?.makeKey() }
        window.layoutIfNeeded()
        try await Task.sleep(for: .milliseconds(100))
        let slider = try #require(findSlider(window))
        slider.sendActions(for: .touchDown)
        #expect(editing == [true])
        slider.sendActions(for: UIControl.Event(rawValue: event))
        #expect(editing == [true, false])
        #expect(position == 20)
    }

    @Test func accessibilitySeekingCommitsThroughTheExistingEditingCallbacks() async throws {
        var position = 20.0
        var editing: [Bool] = []
        let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: BufferedPlaybackSlider(
            value: Binding(get: { position }, set: { position = $0 }), duration: 100, buffered: [0..<60],
            onEditingChanged: { editing.append($0) }).frame(width: 320, height: 44))
        window.makeKeyAndVisible()
        defer { window.isHidden = true; previous?.makeKey() }
        window.layoutIfNeeded()
        try await Task.sleep(for: .milliseconds(100))
        let slider = try #require(findSlider(window))
        slider.accessibilityIncrement()
        #expect(position > 20)
        #expect(editing == [true, false])
        #expect(slider.bounds.height >= 44)
    }

    @Test(arguments: [320.0, 390, 844, 1024], [UIUserInterfaceStyle.dark, .light])
    func renderedBufferDiffersFromUnbufferedTrackAndClears(_ width: Double, _ style: UIUserInterfaceStyle) async throws {
        let slider = BufferedSlider(frame: CGRect(x: 0, y: 0, width: width, height: 44))
        let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIViewController()
        window.overrideUserInterfaceStyle = style
        window.rootViewController?.view.addSubview(slider)
        window.makeKeyAndVisible()
        defer { window.isHidden = true; previous?.makeKey() }
        slider.overrideUserInterfaceStyle = style
        slider.backgroundColor = style == .dark ? .black : .white
        slider.minimumValue = 0; slider.maximumValue = 100; slider.value = 20
        slider.buffered = [0..<60]
        slider.layoutIfNeeded()
        try await Task.sleep(for: .milliseconds(50))
        let buffered = try brightness(slider, fraction: 0.5)
        let empty = try brightness(slider, fraction: 0.9)
        #expect(abs(buffered - empty) > 0.15)
        #expect(slider.value == 20)
        #expect(slider.isAccessibilityElement)
        slider.buffered = []
        #expect(abs(try brightness(slider, fraction: 0.5) - empty) < 0.02)
        #expect(slider.value == 20)
        let folder = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("buffer-bar-proof")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        slider.buffered = [0..<60]
        try #require(image(slider).pngData()).write(to: folder.appendingPathComponent("slider-\(Int(width))-\(style.rawValue).png"))
    }

    private func image(_ slider: UISlider) -> UIImage {
        slider.setNeedsDisplay(); slider.layoutIfNeeded()
        let format = UIGraphicsImageRendererFormat(); format.scale = 1
        return UIGraphicsImageRenderer(bounds: slider.bounds, format: format).image { context in
            slider.layer.render(in: context.cgContext)
        }
    }

    private func findSlider(_ view: UIView) -> UISlider? {
        if let slider = view as? UISlider { return slider }
        return view.subviews.lazy.compactMap { findSlider($0) }.first
    }

    private func brightness(_ slider: UISlider, fraction: Double) throws -> Double {
        let cg = try #require(image(slider).cgImage)
        let bytes = try #require(cg.dataProvider?.data) as Data
        let track = slider.trackRect(forBounds: slider.bounds)
        let index = Int(track.midY) * cg.bytesPerRow + Int(track.minX + track.width * fraction) * 4
        return Double(UInt16(bytes[index]) + UInt16(bytes[index + 1]) + UInt16(bytes[index + 2])) / (3 * 255)
    }
}
#endif
