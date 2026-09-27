import SwiftUI

struct DetailLoadingState: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            #if os(tvOS)
            RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface).frame(height: 520)
                .overlay(alignment: .bottomLeading) {
                    VStack(alignment: .leading, spacing: 16) {
                        line(340, 52)
                        line(360, 20)
                        line(580, 22)
                        line(500, 22)
                        Capsule().fill(KinoTheme.raised).frame(width: 280, height: 58)
                    }
                    .padding(40)
                }
            line(140, 28)
            HStack(spacing: 18) {
                ForEach(0..<4) { _ in
                    RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface)
                        .aspectRatio(2 / 3, contentMode: .fit).frame(width: 230)
                }
            }
            #else
            CinemaHeroLayout {
                RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface)
                    .aspectRatio(16 / 9, contentMode: .fit)
            } information: {
                VStack(alignment: .leading, spacing: 16) {
                    line(260, 42)
                    line(160, 18)
                    line(100, 14)
                    Capsule().fill(KinoTheme.raised).frame(height: 50)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            VStack(alignment: .leading, spacing: 20) {
                line(340, 20)
                line(260, 16)
                line(180, 16)
            }
            .padding(.top, 8)
            #endif
        }
    }

    private func line(_ width: CGFloat, _ height: CGFloat) -> some View {
        RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(maxWidth: width).frame(height: height)
    }
}
