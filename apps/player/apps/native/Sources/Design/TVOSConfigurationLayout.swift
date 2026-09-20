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
                TVOSConfigurationBrand(title: title, symbol: symbol)
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

    var body: some View {
        VStack(alignment: .leading, spacing: 24) {
            ZStack(alignment: .bottomLeading) {
                Image("CinemaSail")
                    .resizable()
                    .scaledToFill()
                    .frame(maxWidth: .infinity)
                    .frame(height: 220)
                    .clipped()
                LinearGradient(
                    colors: [.clear, KinoTheme.background.opacity(0.94)],
                    startPoint: .top,
                    endPoint: .bottom
                )
                VStack(alignment: .leading, spacing: 8) {
                    Image(systemName: "sailboat.fill")
                        .font(.system(size: 44, weight: .semibold))
                        .foregroundStyle(KinoTheme.signal)
                    Text("KINOSAIL")
                        .font(.system(size: 26, weight: .bold, design: .rounded))
                        .tracking(2.5)
                        .foregroundStyle(KinoTheme.text)
                }
                .padding(24)
            }
            .clipShape(.rect(cornerRadius: 24))

            VStack(alignment: .leading, spacing: 8) {
                Image(systemName: symbol)
                    .font(.title2.bold())
                    .foregroundStyle(KinoTheme.signal)
                Text(title)
                    .font(.title2.bold())
                    .foregroundStyle(KinoTheme.text)
                    .fixedSize(horizontal: false, vertical: true)
                Text("Move with the remote. Select with one press.")
                    .font(.body)
                    .foregroundStyle(KinoTheme.muted)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(title). Kinosail settings.")
    }
}
#endif
