import SwiftUI

struct ResumeRows: View {
    let items: [MediaItem]
    var title = "Continue watching"
    var showsAll = true
    @Environment(\.dynamicTypeSize) private var dynamicType
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(alignment: .firstTextBaseline) {
                Text(title).font(.title2.bold()).accessibilityAddTraits(.isHeader)
                Spacer()
                if showsAll {
                    NavigationLink("See all", value: ScreenDestination.library(.history))
                        .font(.callout).frame(minHeight: 44)
                        #if os(tvOS)
                        .buttonStyle(.bordered).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
                        #else
                        .foregroundStyle(KinoTheme.signal)
                        #endif
                }
            }
            LazyVGrid(columns: columns, alignment: .leading, spacing: 16) {
                ForEach(items) { item in ResumeRow(item: item) }
            }
            #if os(tvOS)
            .padding(.vertical, 20)
            #endif
        }
        #if os(tvOS)
        .focusSection()
        #endif
    }
    private var columns: [GridItem] { Self.columns(accessibility: dynamicType.isAccessibilitySize) }
    static func columns(accessibility: Bool) -> [GridItem] {
        #if os(tvOS)
        let minimum: CGFloat = 640
        #else
        let minimum: CGFloat = 420
        #endif
        return [accessibility ? GridItem(.flexible(), alignment: .top)
                : GridItem(.adaptive(minimum: minimum), spacing: 24, alignment: .top)]
    }
}

private struct ResumeRow: View {
    let item: MediaItem
    @Environment(\.dynamicTypeSize) private var dynamicType
    var body: some View {
        NavigationLink(value: item.playingDestination) {
            HStack(spacing: 12) {
                if !dynamicType.isAccessibilitySize {
                    let usesBackdrop = !item.backdrop.isEmpty
                    Artwork(path: usesBackdrop ? item.backdrop : item.poster, symbol: item.kind.symbol,
                            ratio: usesBackdrop ? 16 / 9 : item.isAudio ? 1 : 2 / 3,
                            dimension: 800, isBackdrop: usesBackdrop)
                        .frame(width: artworkWidth).clipShape(.rect(cornerRadius: 8))
                }
                VStack(alignment: .leading, spacing: 4) {
                    Text(item.title).font(.headline).foregroundStyle(KinoTheme.text)
                        .lineLimit(dynamicType.isAccessibilitySize ? nil : 2)
                    if !item.subtitle.isEmpty { Text(item.subtitle).font(.caption).foregroundStyle(KinoTheme.muted) }
                    WatchPosition(item: item, compact: true)
                }.frame(maxWidth: .infinity, alignment: .leading)
                Image(systemName: "play.fill").foregroundStyle(KinoTheme.text).accessibilityHidden(true)
            }
            .frame(maxWidth: .infinity, minHeight: 80, alignment: .leading)
            .contentShape(.rect)
            #if os(tvOS)
            .padding(12)
            #endif
        }
        #if os(iOS)
        .buttonStyle(.plain)
        #else
        .buttonStyle(.card)
        #endif
        .accessibilityElement(children: .combine)
    }
    private var artworkWidth: CGFloat {
        #if os(tvOS)
        196
        #else
        112
        #endif
    }
}
