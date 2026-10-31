import SwiftUI

struct ListedShowsShelf: View {
    @Binding var count: Int
    @Environment(AppSession.self) private var session
    @State private var shows: [MediaItem] = []

    var body: some View {
        Group {
            if !shows.isEmpty { MediaShelf(title: "Shows", items: shows, opensShows: true) }
        }
        .task(id: "\(session.profileKey ?? ""):\(session.contentRevision)") {
            guard let client = session.client, !session.usesDeviceLibrary else { return }
            shows = (try? await client.listedShows()) ?? shows
            count = shows.count
        }
    }
}
