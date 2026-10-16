import SwiftUI

struct SkeletonLine: View {
    let width: CGFloat
    let height: CGFloat

    var body: some View {
        RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(maxWidth: width).frame(height: height)
            .frame(height: height == 28 ? KinoTheme.shelfHeadingHeight : nil)
    }
}

struct TVHomeSpotlightSkeleton: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            SkeletonLine(width: 520, height: 66)
            SkeletonLine(width: 300, height: 30)
            SkeletonLine(width: 900, height: 30)
            SkeletonLine(width: 640, height: 30)
        }
        .padding(.top, 40)
        .accessibilityHidden(true)
    }
}

struct TVHomeBrowseSkeleton: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            SkeletonLine(width: 130, height: 28)
            ScrollView(.horizontal) {
                HStack(spacing: KinoTheme.shelfSpacing) {
                    ForEach(0..<4) { _ in
                        HStack(spacing: 18) {
                            RoundedRectangle(cornerRadius: 12).fill(KinoTheme.raised).frame(width: 60, height: 60)
                            SkeletonLine(width: 120, height: 20)
                            Spacer(minLength: 0)
                        }
                        .padding(.horizontal, 24)
                        .frame(width: 400, height: 120)
                        .background(KinoTheme.surface, in: .rect(cornerRadius: 12))
                    }
                }
                .padding(.vertical, 24)
            }
            .scrollIndicators(.hidden).scrollDisabled(true)
        }
        .accessibilityHidden(true)
    }
}
