import SwiftUI

struct LaunchReveal<Content: View>: View {
    let ready: Bool
    let stage: LaunchStage
    @ViewBuilder let content: () -> Content
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var finished = false
    @State private var markScale: CGFloat = 1
    @State private var markOpacity: Double = 1
    @State private var glowOpacity: Double = 0
    @State private var breathing = false
    @State private var curtainOpacity: Double = 1
    @State private var contentScale: CGFloat = KinoMotion.contentStartScale
    @State private var shade: Double = KinoMotion.contentStartShade
    @State private var depthDone = false
    @State private var canvas: CGSize = .zero

    #if os(tvOS)
    private static var markSize: CGSize { CGSize(width: 160, height: 206) }
    #else
    private static var markSize: CGSize { CGSize(width: 100, height: 129) }
    #endif
    private static var anchor: UnitPoint { UnitPoint(x: 0.28, y: 0.4) }
    private static var inradius: CGFloat { 0.2795 }

    var body: some View {
        content()
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .overlay {
                if !depthDone && !reduceMotion {
                    Color.black.opacity(shade).ignoresSafeArea().allowsHitTesting(false)
                }
            }
            .scaleEffect(reduceMotion ? 1 : contentScale)
            .accessibilityHidden(!finished)
            .background { if !finished { Color.black.ignoresSafeArea() } }
            .environment(\.launchStage, stage)
            .overlay {
                if !finished {
                    curtain
                        .ignoresSafeArea()
                        .onGeometryChange(for: CGSize.self) { $0.size } action: { canvas = $0 }
                        .allowsHitTesting(true)
                }
            }
            .task(id: ready) {
                guard ready, !finished else { return }
                await reveal()
            }
    }

    private var curtain: some View {
        ZStack {
            Color.black
            mark.blendMode(.destinationOut)
        }
        .compositingGroup()
        .opacity(curtainOpacity)
        .overlay { glow }
        .overlay { mark.opacity(markOpacity) }
        .accessibilityElement()
        .accessibilityLabel("Kinosail Player")
    }

    private var mark: some View {
        Image("LaunchMark")
            .resizable()
            .scaledToFit()
            .frame(width: Self.markSize.width, height: Self.markSize.height)
            .scaleEffect(markScale * (breathing ? 0.965 : 1), anchor: Self.anchor)
    }

    private var glow: some View {
        RadialGradient(colors: [KinoTheme.signal.opacity(0.32), KinoTheme.signal.opacity(0)],
                       center: .center, startRadius: 0, endRadius: Self.markSize.height * 1.3)
            .frame(width: Self.markSize.height * 2.6, height: Self.markSize.height * 2.6)
            .scaleEffect(breathing ? 1.1 : 0.94)
            .opacity(glowOpacity)
            .allowsHitTesting(false)
            .accessibilityHidden(true)
    }

    private var openScale: CGFloat {
        let size = canvas == .zero ? CGSize(width: 1920, height: 1920) : canvas
        let origin = CGPoint(x: size.width / 2 + (Self.anchor.x - 0.5) * Self.markSize.width,
                             y: size.height / 2 + (Self.anchor.y - 0.5) * Self.markSize.height)
        let reach = max(origin.x, size.width - origin.x).squared + max(origin.y, size.height - origin.y).squared
        return reach.squareRoot() / (Self.inradius * Self.markSize.width) * 1.1
    }

    private func reveal() async {
        if reduceMotion {
            stage.arrive()
            withAnimation(KinoMotion.reducedFade) { curtainOpacity = 0; markOpacity = 0 }
            await pause(.milliseconds(250))
            finished = true
            depthDone = true
            stage.settle()
            return
        }
        withAnimation(KinoMotion.markSettle) { markScale = 0.84 }
        withAnimation(KinoMotion.glowFade) { glowOpacity = 1 }
        await pause(KinoMotion.settleLead)
        if !stage.contentReady {
            withAnimation(KinoMotion.markBreathe) { breathing = true }
            await waitForContent()
            withAnimation(KinoMotion.markSettle) { breathing = false }
        }
        stage.arrive()
        withAnimation(KinoMotion.portal) { markScale = openScale }
        withAnimation(KinoMotion.markFade.delay(0.06)) { markOpacity = 0; glowOpacity = 0 }
        withAnimation(KinoMotion.contentSettle.delay(0.14)) { contentScale = 1; shade = 0 }
        withAnimation(KinoMotion.curtainFade.delay(0.4)) { curtainOpacity = 0 }
        await pause(KinoMotion.portalLength)
        finished = true
        await pause(.milliseconds(420))
        depthDone = true
        stage.settle()
    }

    private func waitForContent() async {
        let deadline = ContinuousClock.now + KinoMotion.contentWaitBudget
        while !stage.contentReady && ContinuousClock.now < deadline && !Task.isCancelled {
            await pause(.milliseconds(40))
        }
    }

    private func pause(_ duration: Duration) async {
        try? await Task.sleep(for: duration)
    }
}

private extension CGFloat {
    var squared: CGFloat { self * self }
}
