import SwiftUI

struct MediaLinkScreen: View {
    let link: MediaLink
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    var body: some View {
        Group {
            if link.scope != session.profileKey {
                ContentUnavailableView("Profile changed", systemImage: "person.crop.circle", description: Text("Open this title from your current library."))
            } else {
                switch link.action {
                case .search: LibraryScreen(searchMode: true, initialQuery: link.value)
                case .detail: DetailScreen(itemID: link.value)
                case .play:
                    ResourceView(identity: link.id, loadingLayout: .playback, revalidates: false, load: { _ in
                        guard let client = session.client else { throw ClientError.unavailable }
                        return try await client.item(id: link.value, policy: .reload)
                    }) { item in
                        if item.isAudio { AudioPlayerScreen(itemID: item.id) }
                        else if item.kind == .video { PlaybackScreen(itemID: item.id) }
                        else { DetailScreen(itemID: item.id) }
                    }
                }
            }
        }
        .toolbar { ToolbarItem(placement: .cancellationAction) { Button("Done") { dismiss() } } }
        .navigationDestination(for: ScreenDestination.self) { DestinationScreen(destination: $0) }
    }
}
