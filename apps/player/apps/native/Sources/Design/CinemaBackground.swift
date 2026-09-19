import SwiftUI

// Content supplies its loaded artwork through the preference, so loading,
// empty and failed resources cannot leave a previous title behind.
private struct CinemaBackdropKey: PreferenceKey {
    static let defaultValue = ""
    static func reduce(value: inout String, nextValue: () -> String) {
        let next = nextValue()
        if !next.isEmpty { value = next }
    }
}

extension View {
    @ViewBuilder func cinemaBackdrop(path: String) -> some View {
        #if os(tvOS)
        preference(key: CinemaBackdropKey.self, value: path)
        #else
        self
        #endif
    }

    @ViewBuilder func cinemaBackground() -> some View {
        #if os(tvOS)
        backgroundPreferenceValue(CinemaBackdropKey.self) { path in
            CinemaBackground(path: path)
        }
        #else
        background(KinoTheme.background)
        #endif
    }
}

#if os(tvOS)
private struct CinemaBackground: View {
    let path: String
    @Environment(\.colorSchemeContrast) private var contrast
    @Environment(\.accessibilityReduceTransparency) private var reduceTransparency

    var body: some View {
        ZStack {
            KinoTheme.background
            if contrast != .increased && !reduceTransparency && !path.isEmpty {
                Artwork(path: path, dimension: 1600, fillsFrame: true, isBackdrop: true)
                    // Keep scenery visible above the hero, then settle into a
                    // quiet canvas behind metadata, controls and later shelves.
                    .overlay {
                        LinearGradient(stops: [
                            .init(color: .black.opacity(0.50), location: 0),
                            .init(color: .black.opacity(0.80), location: 0.22),
                            .init(color: KinoTheme.background.opacity(0.92), location: 0.60),
                            .init(color: KinoTheme.background, location: 0.90)
                        ], startPoint: .top, endPoint: .bottom)
                    }
            }
        }
        .ignoresSafeArea()
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}
#endif
