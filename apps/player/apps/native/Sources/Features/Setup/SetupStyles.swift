import SwiftUI

struct UserAvatar: View {
    let name: String
    let image: UIImage?
    let size: CGFloat

    var body: some View {
        ZStack {
            if let image {
                Image(uiImage: image).resizable().scaledToFill()
            } else {
                LinearGradient(colors: palette, startPoint: .topLeading, endPoint: .bottomTrailing)
                Text(String(name.prefix(1)).uppercased())
                    .font(.system(size: size * 0.42, weight: .bold, design: .rounded)).foregroundStyle(.white)
            }
        }
        .frame(width: size, height: size)
        .clipShape(.circle)
        .overlay(Circle().strokeBorder(.white.opacity(0.14), lineWidth: 1))
        .accessibilityHidden(true)
    }

    private var palette: [Color] {
        let hues: [Double] = [0.08, 0.33, 0.55, 0.72, 0.9]
        let hue = hues[Int(name.unicodeScalars.reduce(0) { $0 &+ $1.value } % UInt32(hues.count))]
        return [Color(hue: hue, saturation: 0.55, brightness: 0.85), Color(hue: hue, saturation: 0.7, brightness: 0.5)]
    }
}

extension View {
    @ViewBuilder func card() -> some View {
        #if os(iOS)
        padding(20)
            .frame(maxWidth: .infinity)
            .background(KinoTheme.surface.opacity(0.86), in: .rect(cornerRadius: 24, style: .continuous))
            .overlay(RoundedRectangle(cornerRadius: 24, style: .continuous).strokeBorder(.white.opacity(0.06), lineWidth: 1))
        #else
        padding(28).frame(maxWidth: .infinity).background(KinoTheme.surface.opacity(0.7), in: .rect(cornerRadius: 28))
        #endif
    }

    @ViewBuilder func fieldSurface() -> some View {
        #if os(iOS)
        padding(.horizontal, 16)
            .frame(minHeight: 52)
            .background(KinoTheme.raised.opacity(0.7), in: .rect(cornerRadius: 14, style: .continuous))
        #else
        self
        #endif
    }
}

extension KinoTheme {
    static var actionInk: Color {
        #if os(tvOS)
        tvOSPrimaryInk
        #else
        signalInk
        #endif
    }
}

extension View {
    @ViewBuilder func primaryCapsule() -> some View {
        #if os(tvOS)
        buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.tvOSPrimaryFill)
        #else
        buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal)
        #endif
    }
}
