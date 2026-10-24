import SwiftUI

struct DetailLoadingState: View {
    @ScaledMetric(relativeTo: .largeTitle) private var titleHeight = 82.0
    @ScaledMetric(relativeTo: .callout) private var metadataHeight = 34.0
    @ScaledMetric(relativeTo: .body) private var bodyHeight = 32.0
    @ScaledMetric(relativeTo: .body) private var actionHeight = 92.0
    var body: some View {
        VStack(alignment: .leading, spacing: sectionSpacing) {
            #if os(tvOS)
            TVMovieHeroLayout { VStack(alignment: .leading, spacing: 14) {
                line(440, titleHeight)
                line(360, metadataHeight)
                VStack(alignment: .leading, spacing: 8) {
                    line(580, bodyHeight)
                    line(500, bodyHeight)
                }
                HStack(spacing: 12) { line(620, 6); line(70, 24) }
                Capsule().fill(KinoTheme.raised).frame(width: 240, height: actionHeight)
                HStack(spacing: 16) {
                    ForEach([206.0, 262.0, 222.0], id: \.self) { width in
                        Capsule().fill(KinoTheme.raised).frame(width: width, height: actionHeight)
                    }
                }
            } }
                .tvMovieHeroArtwork()
                .tvMovieHeroBleed()
            VStack(alignment: .leading, spacing: 16) {
                Text("Cast").font(.title2.bold()).hidden()
                    .overlay(alignment: .leading) { line(140, 28) }
                ScrollView(.horizontal) {
                    HStack(spacing: 18) {
                        ForEach(0..<6) { _ in
                            RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface)
                                .aspectRatio(2 / 3, contentMode: .fit).frame(width: 230)
                        }
                    }.padding(.horizontal, 24).padding(.vertical, 24)
                }
                .scrollIndicators(.hidden).scrollDisabled(true)
            }
            #else
            SpotlightSkeleton(eyebrow: false)
            VStack(alignment: .leading, spacing: 20) {
                line(340, 20)
                line(260, 16)
                line(180, 16)
            }
            .padding(.top, 8)
            #endif
        }
    }

    private var sectionSpacing: CGFloat {
        #if os(tvOS)
        28
        #else
        20
        #endif
    }

    private func line(_ width: CGFloat, _ height: CGFloat) -> some View {
        RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(maxWidth: width).frame(height: height)
    }
}
