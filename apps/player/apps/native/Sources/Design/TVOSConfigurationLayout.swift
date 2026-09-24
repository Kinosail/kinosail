import SwiftUI

/// Gives tvOS forms and lists a stable visual anchor without changing the iOS layout.
struct TVOSConfigurationLayout<Content: View>: View {
    let title: String
    let symbol: String
    let layoutID: String
    @ViewBuilder private let content: () -> Content

    init(title: String, symbol: String, id: String? = nil, @ViewBuilder content: @escaping () -> Content) {
        self.title = title
        self.symbol = symbol
        self.layoutID = id ?? "configuration.\(title)"
        self.content = content
    }

    var body: some View {
        #if os(tvOS)
        GeometryReader { proxy in
            let brandWidth = min(280, max(220, proxy.size.width * 0.22))
            let contentWidth = min(720, max(560, proxy.size.width * 0.46))
            let spacing = min(48, max(28, proxy.size.width * 0.03))

            HStack(alignment: .top, spacing: spacing) {
                TVOSConfigurationBrand(title: title, symbol: symbol, width: brandWidth)
                    .frame(width: brandWidth)
                content()
                    .focusSection()
                    .background(KinoTheme.surface.opacity(0.78))
                    .frame(width: contentWidth)
                    .id(layoutID)
                    .accessibilityIdentifier(layoutID)
            }
            .frame(maxWidth: 1120, maxHeight: .infinity, alignment: .center)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .background(KinoTheme.background)
        .cinemaBackground()
        #else
        content()
        #endif
    }
}

extension View {
    @ViewBuilder
    func tvOSConfigurationLayout(title: String, symbol: String, id: String? = nil) -> some View {
        #if os(tvOS)
        TVOSConfigurationLayout(title: title, symbol: symbol, id: id) { self }
        #else
        self
        #endif
    }
}

#if os(tvOS)
private struct TVOSConfigurationBrand: View {
    let title: String
    let symbol: String
    let width: CGFloat

    var body: some View {
        VStack(alignment: .leading, spacing: 24) {
            ZStack {
                Image("CinemaSail")
                    .resizable()
                    .scaledToFill()
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                    .clipped()
                LinearGradient(
                    colors: [KinoTheme.background.opacity(0.18), KinoTheme.background.opacity(0.94)],
                    startPoint: .top,
                    endPoint: .bottom
                )
                LinearGradient(
                    colors: [KinoTheme.background.opacity(0.9), .clear],
                    startPoint: .leading,
                    endPoint: .trailing
                )
                VStack(alignment: .leading, spacing: 24) {
                    VStack(alignment: .leading, spacing: 8) {
                        Image("KinosailMark")
                            .resizable()
                            .scaledToFit()
                            .frame(width: 44, height: 56)
                        Text("KINOSAIL")
                            .font(.system(size: 26, weight: .bold, design: .rounded))
                            .tracking(2.5)
                            .foregroundStyle(KinoTheme.text)
                    }
                    VStack(alignment: .leading, spacing: 8) {
                        Image(systemName: symbol)
                            .font(.title2.bold())
                            .foregroundStyle(KinoTheme.signal)
                        Text(title)
                            .font(.title2.bold())
                            .foregroundStyle(KinoTheme.text)
                            .frame(width: max(180, width - 32), alignment: .leading)
                            .lineLimit(3)
                            .minimumScaleFactor(0.72)
                            .allowsTightening(true)
                        Text("Move with the remote. Select with one press.")
                            .font(.body)
                            .foregroundStyle(KinoTheme.muted)
                            .frame(width: max(180, width - 32), alignment: .leading)
                    }
                }
                .padding(32)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(title). Kinosail settings.")
    }
}
#endif
