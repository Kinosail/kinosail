import SwiftUI

struct BufferedPlaybackTrack: View {
    let value: Double
    let duration: Double
    let buffered: [Range<Double>]

    var body: some View {
        Canvas { context, size in
            let total = duration.isFinite && duration > 0 ? duration : 1
            let bounds = CGRect(origin: .zero, size: size)
            context.clip(to: Path(roundedRect: bounds, cornerRadius: size.height / 2))
            context.fill(Path(bounds), with: .color(.primary.opacity(0.28)))
            for range in buffered where range.lowerBound.isFinite && range.upperBound.isFinite {
                let start = min(total, max(0, range.lowerBound)) / total * size.width
                let end = min(total, max(0, range.upperBound)) / total * size.width
                context.fill(Path(CGRect(x: start, y: 0, width: max(0, end - start), height: size.height)), with: .color(.primary.opacity(0.72)))
            }
            let played = value.isFinite ? min(total, max(0, value)) / total * size.width : 0
            context.fill(Path(CGRect(x: 0, y: 0, width: played, height: size.height)), with: .color(KinoTheme.signal))
        }
        .frame(height: 6)
        .accessibilityElement(children: .ignore)
    }
}

#if os(iOS)
import UIKit

struct BufferedPlaybackSlider: UIViewRepresentable {
    @Binding var value: Double
    let duration: Double
    let buffered: [Range<Double>]
    var video = false
    var onEditingChanged: (Bool) -> Void

    func makeCoordinator() -> Coordinator { Coordinator(self) }

    func makeUIView(context: Context) -> BufferedSlider {
        let slider = BufferedSlider()
        slider.addTarget(context.coordinator, action: #selector(Coordinator.changed(_:)), for: .valueChanged)
        slider.addTarget(context.coordinator, action: #selector(Coordinator.began), for: .touchDown)
        slider.addTarget(context.coordinator, action: #selector(Coordinator.ended), for: [.touchUpInside, .touchUpOutside, .touchCancel])
        return slider
    }

    func updateUIView(_ slider: BufferedSlider, context: Context) {
        context.coordinator.parent = self
        let total = duration.isFinite && duration > 0 ? duration : 1
        slider.maximumValue = Float(total)
        slider.value = Float(value.isFinite ? min(total, max(0, value)) : 0)
        slider.buffered = buffered
        slider.ink = video ? .white : .label
        slider.thumbTintColor = video ? .white : nil
        slider.isEnabled = context.environment.isEnabled && duration > 0
        slider.accessibilityValue = "\(value.clock) of \(duration > 0 ? duration.clock : "unknown duration")"
        slider.setNeedsDisplay()
    }

    final class Coordinator: NSObject {
        var parent: BufferedPlaybackSlider
        init(_ parent: BufferedPlaybackSlider) { self.parent = parent }
        @objc func began() { parent.onEditingChanged(true) }
        @objc func ended() { parent.onEditingChanged(false) }
        @objc func changed(_ slider: UISlider) {
            // VoiceOver and keyboard changes have no touch-down/up events.
            if !slider.isTracking { parent.onEditingChanged(true) }
            parent.value = Double(slider.value)
            slider.setNeedsDisplay()
            if !slider.isTracking { parent.onEditingChanged(false) }
        }
    }
}

final class BufferedSlider: UISlider {
    var buffered: [Range<Double>] = [] { didSet { setNeedsDisplay() } }
    var ink: UIColor = .label
    override var intrinsicContentSize: CGSize { CGSize(width: super.intrinsicContentSize.width, height: 44) }

    override init(frame: CGRect) {
        super.init(frame: frame)
        minimumTrackTintColor = .clear; maximumTrackTintColor = .clear
        isAccessibilityElement = true; accessibilityTraits = .adjustable; accessibilityLabel = "Playback position"
    }
    required init?(coder: NSCoder) { fatalError("init(coder:) has not been implemented") }

    override func accessibilityIncrement() { adjust(by: 1) }
    override func accessibilityDecrement() { adjust(by: -1) }
    private func adjust(by direction: Float) {
        guard isEnabled else { return }
        value = min(maximumValue, max(minimumValue, value + direction * (maximumValue - minimumValue) / 100))
        sendActions(for: .valueChanged)
    }

    override func draw(_ rect: CGRect) {
        super.draw(rect)
        guard let context = UIGraphicsGetCurrentContext() else { return }
        let track = trackRect(forBounds: bounds)
        context.addPath(UIBezierPath(roundedRect: track, cornerRadius: track.height / 2).cgPath)
        context.clip()
        let color = ink.resolvedColor(with: traitCollection)
        color.withAlphaComponent(0.28).setFill(); context.fill(track)
        let total = Double(maximumValue - minimumValue)
        guard total > 0 else { return }
        color.withAlphaComponent(traitCollection.accessibilityContrast == .high ? 1 : 0.72).setFill()
        for range in buffered where range.lowerBound.isFinite && range.upperBound.isFinite {
            let start = min(total, max(0, range.lowerBound)) / total * track.width
            let end = min(total, max(0, range.upperBound)) / total * track.width
            context.fill(CGRect(x: track.minX + start, y: track.minY, width: max(0, end - start), height: track.height))
        }
        UIColor(KinoTheme.signal).setFill()
        context.fill(CGRect(x: track.minX, y: track.minY, width: min(1, max(0, Double(value - minimumValue) / total)) * track.width, height: track.height))
    }
}
#endif
