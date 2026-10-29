import SwiftUI

struct LaunchReveal<Content: View>: View {
    let ready: Bool
    @ViewBuilder let content: () -> Content
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var finished = false
    @State private var markScale: CGFloat = 1
    @State private var markOpacity: Double = 1
    @State private var curtainOpacity: Double = 1
    @State private var contentScale: CGFloat = Self.startScale
    @State private var contentOpacity: Double = Self.startOpacity
    @State private var canvas: CGSize = .zero

    #if os(tvOS)
    private static var markSize: CGSize { CGSize(width: 160, height: 206) }
    private static var startScale: CGFloat { 1.04 }
    private static var startOpacity: Double { 0.02 }
    #else
    private static var markSize: CGSize { CGSize(width: 100, height: 129) }
    private static var startScale: CGFloat { 1.08 }
    private static var startOpacity: Double { 1 }
    #endif
    private static var anchor: UnitPoint { UnitPoint(x: 0.28, y: 0.4) }
    private static var inradius: CGFloat { 0.2795 }

    var body: some View {
        content()
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .scaleEffect(finished || reduceMotion ? 1 : contentScale)
            .opacity(finished || reduceMotion ? 1 : contentOpacity)
            .accessibilityHidden(!finished)
            .background { if !finished { Color.black.ignoresSafeArea() } }
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
            #if !os(tvOS)
            mark.blendMode(.destinationOut)
            #endif
        }
        .compositingGroup()
        .opacity(curtainOpacity)
        .overlay { mark.opacity(markOpacity) }
        .accessibilityElement()
        .accessibilityLabel("Kinosail Player")
    }

    private var mark: some View {
        Image("LaunchMark")
            .resizable()
            .scaledToFit()
            .frame(width: Self.markSize.width, height: Self.markSize.height)
            .scaleEffect(markScale, anchor: Self.anchor)
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
            withAnimation(.easeOut(duration: 0.25)) { curtainOpacity = 0; markOpacity = 0 }
            await pause(250)
            finished = true
            return
        }
        #if os(tvOS)
        withAnimation(.smooth(duration: 0.24)) { markScale = 0.92 }
        await pause(200)
        withAnimation(.smooth(duration: 0.55)) { markScale = 1.35; markOpacity = 0 }
        withAnimation(.smooth(duration: 0.5).delay(0.12)) { curtainOpacity = 0 }
        withAnimation(.smooth(duration: 0.7).delay(0.12)) { contentScale = 1; contentOpacity = 1 }
        await pause(860)
        finished = true
        #else
        withAnimation(.spring(duration: 0.28, bounce: 0.25)) { markScale = 0.84 }
        await pause(210)
        withAnimation(.timingCurve(0.7, 0, 0.84, 0.3, duration: 0.6)) { markScale = openScale }
        withAnimation(.easeIn(duration: 0.18).delay(0.06)) { markOpacity = 0 }
        withAnimation(.smooth(duration: 0.8).delay(0.14)) { contentScale = 1 }
        withAnimation(.easeOut(duration: 0.24).delay(0.4)) { curtainOpacity = 0 }
        await pause(680)
        finished = true
        #endif
    }

    private func pause(_ milliseconds: Int) async {
        try? await Task.sleep(for: .milliseconds(milliseconds))
    }
}

private extension CGFloat {
    var squared: CGFloat { self * self }
}
