import SwiftUI

struct DetailMoreMenu: View {
    enum Target {
        case item(MediaItem)
        case show(id: String, episodes: [MediaItem])
    }

    let target: Target
    let busy: Bool
    var goToShow: (() -> Void)?
    let change: (@escaping @MainActor (ServerClient) async throws -> Void) -> Void
    @ScaledMetric(relativeTo: .body) private var height = 44

    var body: some View {
        Menu {
            if let goToShow { Button("Go to show", systemImage: "tv", action: goToShow) }
            switch target {
            case .item(let item): itemActions(item)
            case .show(let id, let episodes): showActions(id: id, episodes: episodes)
            }
        } label: {
            Image(systemName: "ellipsis")
                #if os(tvOS)
                .frame(minWidth: height, minHeight: height)
                #else
                .frame(minWidth: 28, minHeight: 28)
                #endif
        }
        #if os(tvOS)
        .tvCapsule()
        #else
        .buttonStyle(.glass).buttonBorderShape(.circle)
        #endif
        .accessibilityLabel("More")
        .disabled(busy)
    }

    @ViewBuilder private func itemActions(_ item: MediaItem) -> some View {
        if item.kind == .video {
            let watched = item.progress.watched
            Button(watched ? "Mark as unwatched" : "Mark as watched", systemImage: watched ? "eye.slash" : "checkmark.circle") {
                change { try await $0.setWatched(itemID: item.id, watched: !watched) }
            }
            if !item.showID.isEmpty, item.season > 0 {
                Button("Mark season \(item.season) as watched", systemImage: "checkmark.circle.fill") {
                    change { try await $0.setWatched(showID: item.showID, season: item.season, watched: true) }
                }
            }
        }
        if item.progress.seconds > 0 || item.progress.watched {
            Button("Remove from Continue watching", systemImage: "minus.circle", role: .destructive) {
                change { try await $0.dismissContinueWatching(itemID: item.id) }
            }
        }
    }

    @ViewBuilder private func showActions(id: String, episodes: [MediaItem]) -> some View {
        let allWatched = !episodes.isEmpty && episodes.allSatisfy(\.progress.watched)
        Button(allWatched ? "Mark show as unwatched" : "Mark show as watched", systemImage: allWatched ? "eye.slash" : "checkmark.circle") {
            change { try await $0.setWatched(showID: id, watched: !allWatched) }
        }
    }
}
