#if os(tvOS)
import SwiftUI

struct TVHomeBrowse: View {
    @Environment(\.dynamicTypeSize) private var dynamicType
    let selectTab: (PlayerTab) -> Void
    private let tabs: [PlayerTab] = [.movies, .shows, .music, .audiobooks, .photos]

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Browse").font(.title2.bold()).accessibilityAddTraits(.isHeader)
            ScrollView(.horizontal) {
                HStack(alignment: .top, spacing: KinoTheme.shelfSpacing) {
                    ForEach(tabs) { tab in
                        tile(title: tab.title, icon: icon(for: tab)) { selectTab(tab) }
                    }
                }
                .padding(.horizontal, 24)
                .padding(.vertical, 24)
                .scrollTargetLayout()
            }
            .scrollIndicators(.hidden)
            .scrollTargetBehavior(.viewAligned)
            .scrollClipDisabled()
            .padding(.horizontal, -24)
        }
        .focusSection()
    }

    private func tile(title: String, icon: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: 18) {
                Image(icon).resizable().scaledToFit().frame(width: 60, height: 60).accessibilityHidden(true)
                Text(title).font(.title3.weight(.semibold)).foregroundStyle(KinoTheme.text)
                    .multilineTextAlignment(.leading)
                    .lineLimit(dynamicType.isAccessibilitySize ? 2 : 1).minimumScaleFactor(0.8)
                    .fixedSize(horizontal: false, vertical: true)
                Spacer(minLength: 0)
            }
            .padding(.horizontal, 24)
            .frame(width: 400)
            .frame(minHeight: 120)
            .background(LinearGradient(colors: [KinoTheme.raised, KinoTheme.surface],
                                       startPoint: .topLeading, endPoint: .bottomTrailing))
        }
        .buttonStyle(.card)
        .accessibilityLabel(title)
    }

    private func icon(for tab: PlayerTab) -> String {
        switch tab {
        case .movies: "BrowseMovies"
        case .shows: "BrowseShows"
        case .music: "BrowseMusic"
        case .audiobooks: "BrowseAudiobooks"
        case .photos: "BrowsePhotos"
        case .collections: "BrowseCollections"
        default: tab.symbol
        }
    }

}
#endif
