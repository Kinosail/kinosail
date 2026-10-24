import SwiftUI

enum ServerBrand: Equatable {
    case kinosail, source(SourceKind)

    var title: String {
        switch self {
        case .kinosail: "Kinosail"
        case .source(let kind): kind.title
        }
    }

    var monogram: String {
        switch self {
        case .kinosail: "K"
        case .source(.jellyfin): "J"
        case .source(.emby): "E"
        }
    }

    var colors: [Color] {
        switch self {
        case .kinosail: [Color(red: 0.77, green: 1, blue: 0.28), Color(red: 0.36, green: 0.62, blue: 0.05)]
        case .source(.jellyfin): [Color(red: 0.67, green: 0.36, blue: 0.84), Color(red: 0.0, green: 0.64, blue: 0.86)]
        case .source(.emby): [Color(red: 0.32, green: 0.71, blue: 0.29), Color(red: 0.12, green: 0.42, blue: 0.16)]
        }
    }

    var ink: Color { self == .kinosail ? KinoTheme.signalInk : .white }
}

struct ServerMonogram: View {
    let brand: ServerBrand
    var size: CGFloat = 52

    var body: some View {
        Text(brand.monogram)
            .font(.system(size: size * 0.46, weight: .heavy, design: .rounded))
            .foregroundStyle(brand.ink)
            .frame(width: size, height: size)
            .background(LinearGradient(colors: brand.colors, startPoint: .topLeading, endPoint: .bottomTrailing),
                        in: .rect(cornerRadius: size * 0.28, style: .continuous))
            .overlay(RoundedRectangle(cornerRadius: size * 0.28, style: .continuous).strokeBorder(.white.opacity(0.18), lineWidth: 1))
            .accessibilityHidden(true)
    }
}

struct ServerCard: View {
    let brand: ServerBrand
    let name: String
    let detail: String
    var badge: String?
    var connected = false

    var body: some View {
        HStack(spacing: 16) {
            ServerMonogram(brand: brand)
            VStack(alignment: .leading, spacing: 4) {
                Text(name).font(.headline).foregroundStyle(KinoTheme.text).lineLimit(2)
                Text(detail).font(.subheadline).foregroundStyle(KinoTheme.muted).lineLimit(1)
                if let badge {
                    Text(badge)
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(KinoTheme.signal)
                        .padding(.top, 2)
                }
            }
            Spacer(minLength: 8)
            Image(systemName: connected ? "checkmark.circle.fill" : "chevron.right")
                .font(connected ? .title3 : .body.weight(.semibold))
                .foregroundStyle(connected ? KinoTheme.signal : KinoTheme.muted)
                .accessibilityHidden(true)
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 16)
        .frame(maxWidth: .infinity, alignment: .leading)
        #if os(iOS)
        .background(KinoTheme.surface.opacity(0.86), in: .rect(cornerRadius: 22, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 22, style: .continuous).strokeBorder(.white.opacity(0.06), lineWidth: 1))
        #endif
        .contentShape(.rect(cornerRadius: 22))
        .accessibilityElement(children: .combine)
        .accessibilityLabel([name, detail, badge, connected ? "Connected" : nil].compactMap { $0 }.joined(separator: ", "))
    }
}

struct ServerCardPlaceholder: View {
    var body: some View {
        HStack(spacing: 16) {
            RoundedRectangle(cornerRadius: 14, style: .continuous).fill(KinoTheme.raised).frame(width: 52, height: 52)
            VStack(alignment: .leading, spacing: 8) {
                Capsule().fill(KinoTheme.raised).frame(width: 150, height: 14)
                Capsule().fill(KinoTheme.raised).frame(width: 96, height: 10)
            }
            Spacer()
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 16)
        .background(KinoTheme.surface.opacity(0.6), in: .rect(cornerRadius: 22, style: .continuous))
        .skeletonShimmer()
    }
}

struct ScanPulse: View {
    let active: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var expanded = false

    var body: some View {
        ZStack {
            if active && !reduceMotion {
                Circle().stroke(KinoTheme.signal.opacity(0.6), lineWidth: 1.5)
                    .scaleEffect(expanded ? 1 : 0.35).opacity(expanded ? 0 : 1)
            }
            Circle().fill(active ? KinoTheme.signal : KinoTheme.muted).frame(width: 8, height: 8)
        }
        .frame(width: 20, height: 20)
        .onAppear { withAnimation(.easeOut(duration: 1.4).repeatForever(autoreverses: false)) { expanded = true } }
        .accessibilityHidden(true)
    }
}

extension View {
    func cardAppearance(reduceMotion: Bool) -> some View {
        transition(reduceMotion ? .opacity : .asymmetric(insertion: .scale(scale: 0.94).combined(with: .opacity).combined(with: .offset(y: 12)),
                                                         removal: .opacity))
    }

    @ViewBuilder func serverCardButtonStyle() -> some View {
        #if os(tvOS)
        buttonStyle(.card)
        #else
        buttonStyle(PressableCardStyle())
        #endif
    }
}
