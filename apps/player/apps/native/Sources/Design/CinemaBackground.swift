import SwiftUI

extension View {
    @ViewBuilder func cinemaBackground() -> some View {
        background(CinemaBackground())
    }
}

private struct CinemaBackground: View {
    @Environment(\.colorScheme) private var colorScheme
    @Environment(\.colorSchemeContrast) private var contrast
    @Environment(\.accessibilityReduceTransparency) private var reduceTransparency

    var body: some View {
        ZStack {
            KinoTheme.background
            if colorScheme == .dark && contrast != .increased && !reduceTransparency {
                GeometryReader { geometry in
                    Image("CinemaSail")
                        .resizable()
                        .scaledToFill()
                        .frame(width: geometry.size.width, height: geometry.size.height, alignment: .trailing)
                        .clipped()
                        .overlay {
                            LinearGradient(stops: [
                                .init(color: .black.opacity(0.68), location: 0),
                                .init(color: .black.opacity(0.78), location: 0.58),
                                .init(color: KinoTheme.background.opacity(0.98), location: 1)
                            ], startPoint: .top, endPoint: .bottom)
                        }
                }
            }
        }
        .ignoresSafeArea()
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}
