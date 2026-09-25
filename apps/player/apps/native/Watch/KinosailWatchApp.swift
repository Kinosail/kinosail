import SwiftUI

@main
struct KinosailWatchApp: App {
    @State private var remote = WatchRemoteSession()
    @State private var heart = MovieHeartTracker()

    var body: some Scene {
        WindowGroup {
            RemoteView()
                .environment(remote)
                .environment(heart)
                .tint(Color(red: 0.77, green: 1, blue: 0.28))
                .task { remote.start() }
        }
    }
}
