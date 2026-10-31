import SwiftUI

enum KinoMotion {
    static let markSettle = Animation.spring(duration: 0.28, bounce: 0.25)
    static let markBreathe = Animation.easeInOut(duration: 0.9).repeatForever(autoreverses: true)
    static let portal = Animation.timingCurve(0.7, 0, 0.84, 0.3, duration: 0.6)
    static let markFade = Animation.easeIn(duration: 0.18)
    static let glowFade = Animation.easeOut(duration: 0.3)
    static let contentSettle = Animation.spring(duration: 0.8, bounce: 0.12)
    static let curtainFade = Animation.easeOut(duration: 0.24)
    static let arrival = Animation.spring(duration: 0.55, bounce: 0.18)
    static let reducedFade = Animation.easeOut(duration: 0.25)

    static let settleLead: Duration = .milliseconds(210)
    static let portalLength: Duration = .milliseconds(680)
    static let contentWaitBudget: Duration = .milliseconds(1200)
    static let arrivalStagger = 0.07
    static let arrivalLift: CGFloat = 36
    static let arrivalStartOpacity = 0.02
    static let contentStartScale: CGFloat = 1.08
    static let contentStartShade = 0.35
}

@MainActor @Observable final class LaunchStage {
    enum Phase { case curtain, arriving, settled }
    private(set) var phase: Phase = .curtain
    private(set) var contentReady = false

    func contentAppeared() { contentReady = true }
    func arrive() { if phase == .curtain { phase = .arriving } }
    func settle() { phase = .settled }
}

private struct LaunchStageKey: EnvironmentKey {
    static let defaultValue: LaunchStage? = nil
}

extension EnvironmentValues {
    var launchStage: LaunchStage? {
        get { self[LaunchStageKey.self] }
        set { self[LaunchStageKey.self] = newValue }
    }
}

private struct LaunchArrival: ViewModifier {
    let order: Int
    @Environment(\.launchStage) private var stage
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var arrived = false

    private var waiting: Bool { stage?.phase == .curtain && !reduceMotion }
    private var shown: Bool { arrived || stage == nil || reduceMotion || stage?.phase == .settled }

    func body(content: Content) -> some View {
        content
            .opacity(shown ? 1 : KinoMotion.arrivalStartOpacity)
            .offset(y: shown ? 0 : KinoMotion.arrivalLift)
            .onAppear { if !waiting { land() } }
            .onChange(of: stage?.phase) { _, phase in if phase != .curtain { land() } }
    }

    private func land() {
        guard !arrived else { return }
        withAnimation(KinoMotion.arrival.delay(Double(order) * KinoMotion.arrivalStagger)) { arrived = true }
    }
}

extension View {
    func launchArrival(_ order: Int) -> some View { modifier(LaunchArrival(order: order)) }
}
