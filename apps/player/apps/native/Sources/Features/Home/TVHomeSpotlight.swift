#if os(tvOS)
import SwiftUI

@MainActor @Observable final class TVHomeSpotlight {
    var featured: MediaItem?
    private(set) var focused: MediaItem?
    var current: MediaItem? { focused ?? featured }

    func show(_ item: MediaItem) { focused = item }

    static func title(_ item: MediaItem) -> String { item.show.isEmpty ? item.title : item.show }

    static func metadata(_ item: MediaItem) -> String {
        let parts = item.show.isEmpty ? [item.year, item.rating, item.genres]
            : ["S\(item.season) E\(item.episode)", ShowSeasonSelection.episodeTitle(item), item.rating]
        return parts.filter { !$0.isEmpty }.joined(separator: " · ")
    }
}

struct TVHomeBackdropKey: PreferenceKey {
    static var defaultValue: String? { nil }
    static func reduce(value: inout String?, nextValue: () -> String?) { value = nextValue() ?? value }
}

struct TVHomeSpotlightBackdrop: View {
    let spotlight: TVHomeSpotlight
    @State private var path: String?

    var body: some View {
        Color.clear
            .preference(key: TVHomeBackdropKey.self, value: path)
            .task(id: spotlight.current?.backdrop) {
                guard let next = spotlight.current?.backdrop, !next.isEmpty else { return }
                if path != nil { try? await Task.sleep(for: .milliseconds(250)) }
                if !Task.isCancelled { path = next }
            }
    }
}

struct TVHomeBackdrop: View {
    let path: String?

    var body: some View {
        if let path, !path.isEmpty {
            GeometryReader { geometry in
                TVBackdrop(path: path, canvas: geometry.size, ambient: true)
            }
            .ignoresSafeArea()
            .allowsHitTesting(false)
            .accessibilityHidden(true)
        }
    }
}

struct TVHomeSpotlightHeader: View {
    let item: MediaItem?
    @Environment(\.dynamicTypeSize) private var dynamicType
    @ScaledMetric(relativeTo: .largeTitle) private var titleSize = 56.0

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(item.map(TVHomeSpotlight.title) ?? " ")
                .font(.system(size: titleSize, weight: .bold, design: .rounded))
                .lineLimit(dynamicType.isAccessibilitySize ? nil : 1)
                .minimumScaleFactor(0.7)
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
