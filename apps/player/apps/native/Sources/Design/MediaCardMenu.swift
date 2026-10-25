#if os(iOS)
import SwiftUI

private struct MediaCardMenu: ViewModifier {
    let item: MediaItem
    let resumes: Bool
    @Environment(AppSession.self) private var session
    @State private var saved = 0
    @State private var failed = 0

    private var resumable: Bool { item.progress.seconds > 0 && !item.progress.watched }

    func body(content: Content) -> some View {
        content
            .contextMenu {
                if item.kind == .video {
                    Button(resumable ? "Resume" : "Play", systemImage: "play.fill") {
                        session.presentVideo(item.id, preview: item)
                    }
                } else if item.isAudio {
                    NavigationLink(value: item.playingDestination) { Label("Play", systemImage: "play.fill") }
                }
                if resumes || item.kind != .show {
                    NavigationLink(value: item.destination) { Label("Details", systemImage: "info.circle") }
                }
                if item.kind == .video, !item.showID.isEmpty {
                    NavigationLink(value: ScreenDestination.show(item.showID)) { Label("Go to show", systemImage: "tv") }
                }
                if !session.usesDeviceLibrary, item.kind != .show, item.kind != .photo {
                    Button("Add to My List", systemImage: "plus") {
                        change { client in _ = try await client.setListed(itemID: item.id, listed: true) }
                    }
                }
                if !session.usesDeviceLibrary, resumes, item.progress.seconds > 0 || item.progress.watched {
                    Button("Remove from Continue watching", systemImage: "eye.slash") {
                        change { client in try await client.dismissContinueWatching(itemID: item.id) }
                    }
                }
            }
            .sensoryFeedback(.success, trigger: saved)
            .sensoryFeedback(.error, trigger: failed)
    }

    private func change(_ operation: @escaping @MainActor (ServerClient) async throws -> Void) {
        guard let client = session.client else { return }
        Task {
            do { try await operation(client); saved += 1; session.contentRevision = UUID() }
            catch { failed += 1 }
        }
    }
}

extension View {
    func mediaCardMenu(_ item: MediaItem, resumes: Bool) -> some View {
        modifier(MediaCardMenu(item: item, resumes: resumes))
    }
}
#endif
