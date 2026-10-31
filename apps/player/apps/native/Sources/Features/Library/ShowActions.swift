import SwiftUI

struct ShowActions: View {
    let showID: String
    let episodes: [MediaItem]
    @Environment(AppSession.self) private var session
    @State private var listed: Bool?
    @State private var busy = false
    @ScaledMetric(relativeTo: .body) private var height = 44

    var body: some View {
        Group {
            Button {
                change { client in listed = try await client.setShowListed(showID: showID, listed: listed != true) }
            } label: {
                Label(listed == true ? "In My List" : "My List", systemImage: listed == true ? "checkmark" : "plus")
                    .contentTransition(.symbolEffect(.replace))
                    #if os(tvOS)
                    .frame(minHeight: height)
                    #endif
            }
            #if os(tvOS)
            .tvCapsule()
            #else
            .buttonStyle(SpotlightSecondaryButtonStyle())
            .sensoryFeedback(.success, trigger: listed) { old, new in old != nil && new != nil }
            #endif
            .disabled(busy || listed == nil)
            DetailMoreMenu(target: .show(id: showID, episodes: episodes), busy: busy, change: change)
        }
        .task(id: "\(session.profileKey ?? ""):\(showID):\(session.contentRevision)") {
            guard let client = session.client else { return }
            listed = (try? await client.showListed(showID: showID)) ?? listed
        }
    }

    private func change(_ operation: @escaping @MainActor (ServerClient) async throws -> Void) {
        guard let client = session.client, !busy else { return }
        busy = true
        Task {
            defer { busy = false }
            do { try await operation(client); session.contentRevision = UUID() }
            catch { session.notice = AppSession.message(error) }
        }
    }
}
