#if os(iOS)
import SwiftUI
import UIKit

struct PlaybackLandscapeButton: UIViewRepresentable {
    let orientation: PlaybackOrientation
    let window: () -> UIWindow?
    let enabled: Bool
    let returnsToPrevious: Bool
    let interact: () -> Void

    func makeUIView(context: Context) -> UIButton {
        let button = UIButton(type: .system)
        button.backgroundColor = UIColor(white: 0.16, alpha: 0.95)
        button.tintColor = .white
        button.layer.cornerRadius = 22
        button.addTarget(context.coordinator, action: #selector(Coordinator.activate), for: .touchUpInside)
        return button
    }

    func updateUIView(_ button: UIButton, context: Context) {
        context.coordinator.action = { interact(); orientation.toggle(in: window()) }
        button.isEnabled = enabled
        let symbol = returnsToPrevious ? "arrow.down.right.and.arrow.up.left" : "arrow.up.left.and.arrow.down.right"
        button.setImage(UIImage(systemName: symbol, withConfiguration: UIImage.SymbolConfiguration(pointSize: 20, weight: .semibold)), for: .normal)
        button.accessibilityLabel = returnsToPrevious ? "Return to previous orientation" : "Watch in landscape"
        button.accessibilityIdentifier = "playback.landscape"
        button.alpha = enabled ? 1 : 0.5
    }

    func makeCoordinator() -> Coordinator { Coordinator() }

    @MainActor final class Coordinator: NSObject {
        var action: (() -> Void)?
        @objc func activate() { action?() }
    }
}
#endif
