import SwiftUI

struct OfflinePlaybackScreen: View {
    let downloadID: String
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    #if os(iOS)
    @State private var ready = false
    @State private var failure: String?
    @State private var revision = 0
    #endif
    var body: some View {
        #if os(iOS)
        Group {
            if let failure { RetryState(message: failure) { revision += 1 } }
            else if ready, let item = session.player.currentItem {
                if item.isAudio { AudioPlayerScreen(itemID: item.id) }
                else { PlaybackScreen(itemID: item.id) }
            } else if session.downloads.downloads.first(where: { $0.id == downloadID })?.item.isAudio == true {
                ScrollView { AudioLoadingState(title: "Verifying your download…").padding(KinoTheme.contentPadding) }
            } else {
                ZStack {
                    Color.black.ignoresSafeArea()
                    Text("Verifying your download…").font(.callout).foregroundStyle(.white)
                }.frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .navigationTitle("Offline playback")
        .onChange(of: session.player.currentItem?.id) { _, current in
            if ready && current == nil { dismiss() }
        }
        .task(id: "\(downloadID):\(revision)") {
            ready = false; failure = nil
            guard let client = session.client, let store = session.progress,
                  let download = session.downloads.downloads.first(where: { $0.id == downloadID }) else { failure = "This download is unavailable for the current Viewer Profile."; return }
            do {
                let file = try await session.downloads.verifiedFile(id: downloadID)
                try Task.checkCancellation()
                try await session.player.playOffline(download.item, file: file, downloadID: downloadID, client: client, store: store, preferences: session.downloads.preferences.playback)
                ready = true
            } catch is CancellationError {} catch { failure = AppSession.message(error) }
        }
        #else
        ContentUnavailableView("Offline playback is on iPhone and iPad", systemImage: "iphone")
        #endif
    }
}
