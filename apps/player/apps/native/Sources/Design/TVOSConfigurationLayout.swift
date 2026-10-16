import SwiftUI

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

    #if os(tvOS)
    @Environment(\.tvPanelPresentation) private var panel
    #endif

    var body: some View {
        #if os(tvOS)
        if panel {
            TVPanelSection(title: title, layoutID: layoutID, content: content)
        } else {
            configurationBody
        }
        #else
        content()
        #endif
    }

    #if os(tvOS)
    private var configurationBody: some View {
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
        .cinemaBackground()
    }
    #endif
}

extension View {
    @ViewBuilder
    func configurationNavigationTitle(_ title: String, large: Bool = false) -> some View {
        #if os(tvOS)
        self.navigationTitle("")
        #else
        self.navigationTitle(title)
            .scrollContentBackground(.hidden)
            .frame(maxWidth: 760)
            .frame(maxWidth: .infinity)
            .background(KinoTheme.background)
            .navigationBarTitleDisplayMode(large ? .large : .inline)
        #endif
    }

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
    @ScaledMetric(relativeTo: .title2) private var titleSize = 30.0

    var body: some View {
        VStack(alignment: .leading, spacing: 24) {
            ZStack {
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
                            .font(.system(size: titleSize, weight: .bold, design: .rounded))
                            .foregroundStyle(KinoTheme.text)
                            .frame(width: width - 64, alignment: .leading)
                            .lineLimit(3)
                            .minimumScaleFactor(0.72)
                            .allowsTightening(true)
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

#if os(tvOS)
extension EnvironmentValues {
    @Entry var tvPanelPresentation = false
}

private struct TVPlaybackPanel: ViewModifier {
    func body(content: Content) -> some View {
        HStack(spacing: 0) {
            Spacer(minLength: 0)
            content
                .frame(width: 820)
                .background(KinoTheme.background.opacity(0.94).ignoresSafeArea())
        }
        .background {
            LinearGradient(colors: [.clear, .black.opacity(0.6)], startPoint: .leading, endPoint: .trailing)
                .ignoresSafeArea()
                .accessibilityHidden(true)
        }
        .environment(\.tvPanelPresentation, true)
        .presentationBackground(.clear)
    }
}

private struct TVPanelSection<Content: View>: View {
    let title: String
    let layoutID: String
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(title)
                .font(.title2.bold())
                .foregroundStyle(KinoTheme.text)
                .lineLimit(2)
                .accessibilityAddTraits(.isHeader)
                .padding(.horizontal, 56)
                .padding(.top, 64)
            content()
                .focusSection()
                .id(layoutID)
                .accessibilityIdentifier(layoutID)
        }
        .frame(maxHeight: .infinity, alignment: .top)
        .navigationTitle("")
    }
}

extension View {
    func tvPlaybackPanel() -> some View {
        modifier(TVPlaybackPanel())
    }
}
#endif
