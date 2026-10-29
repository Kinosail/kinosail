#if os(tvOS)
import SwiftUI

@MainActor @Observable final class TVHomeSpotlight {
    private(set) var focused: MediaItem?

    func show(_ item: MediaItem) { focused = item }

    static func title(_ item: MediaItem) -> String { item.show.isEmpty ? item.title : item.show }

    static func metadata(_ item: MediaItem) -> String {
        let parts = item.show.isEmpty ? [item.year, item.rating, item.genres]
            : ["S\(item.season) E\(item.episode)", ShowSeasonSelection.episodeTitle(item), item.rating]
        return parts.filter { !$0.isEmpty }.joined(separator: " · ")
    }
}

struct TVHomeSpotlightHeader: View {
    let spotlight: TVHomeSpotlight
    let fallback: MediaItem?
    private var item: MediaItem? { spotlight.focused ?? fallback }
    @Environment(\.dynamicTypeSize) private var dynamicType
    @ScaledMetric(relativeTo: .largeTitle) private var titleSize = 56.0

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(item.map(TVHomeSpotlight.title) ?? " ")
                .font(.system(size: titleSize, weight: .bold, design: .rounded))
                .lineLimit(dynamicType.isAccessibilitySize ? nil : 1)
                .minimumScaleFactor(0.7)
                .frame(height: dynamicType.isAccessibilitySize ? nil : UIFont.systemFont(ofSize: titleSize, weight: .bold).lineHeight,
                       alignment: .leading)
            Text(item.map(TVHomeSpotlight.metadata) ?? " ")
                .font(.callout.weight(.medium)).foregroundStyle(KinoTheme.muted)
                .lineLimit(1)
            Text(item?.plot ?? " ")
                .font(.body).foregroundStyle(KinoTheme.text.opacity(0.88))
                .mediaLineLimit(2, accessibility: dynamicType.isAccessibilitySize)
                .frame(maxWidth: 860, alignment: .leading)
        }
        .foregroundStyle(KinoTheme.text)
        .frame(maxWidth: 1040, alignment: .leading)
        .padding(.top, 40)
        .accessibilityHidden(true)
    }
}
#endif
