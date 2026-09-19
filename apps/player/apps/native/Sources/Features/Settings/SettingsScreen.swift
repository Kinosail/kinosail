import SwiftUI

struct SettingsScreen: View {
    @Environment(AppSession.self) private var session
    @State private var signingOut = false
    #if os(tvOS)
    @AppStorage(TopShelfPreferences.enabledKey) private var topShelf = true
    #endif
    var body: some View {
        Form {
            Section("Server & Viewer Profile") {
                if let viewer = session.viewer {
                    LabeledContent("Server", value: viewer.serverName)
                    LabeledContent("Viewer Profile", value: viewer.name)
                }
                if let client = session.client { Text(client.server.url.absoluteString).font(.caption).foregroundStyle(.secondary) }
                Button("Change Server", systemImage: "network") { session.showsSetup = true }
                #if os(tvOS)
                Text("Everyone using this Apple TV shares the connected Viewer Profile, including its library access and watch history.")
                    .font(.footnote).foregroundStyle(.secondary)
                #endif
                NavigationLink("Connect a TV", value: ScreenDestination.approval)
            }
            Section("Appearance") {
                NavigationLink("Customize tabs", value: ScreenDestination.tabPreferences)
                #if os(tvOS)
                Toggle("Show titles on Apple TV Home", isOn: $topShelf)
                Text("Show Continue watching, My List and recent titles when Kinosail is selected in the top row. Titles and artwork are visible to anyone using this Apple TV.")
                    .font(.footnote).foregroundStyle(.secondary)
                #endif
            }
            Section("Media") {
                NavigationLink("Playback", value: ScreenDestination.playbackPreferences)
                #if os(iOS)
                NavigationLink("Downloads", value: ScreenDestination.offlinePreferences)
                NavigationLink("Reader", value: ScreenDestination.readerPreferences)
                #endif
                NavigationLink("Progress sync", value: ScreenDestination.progressSync)
            }
            Section {
                Text("Kinosail plays directly from your Server. Your session is stored securely on this device.")
                    .font(.footnote).foregroundStyle(.secondary)
                Button(signingOut ? "Signing out…" : "Sign out", role: .destructive) {
                    signingOut = true
                    Task { await session.disconnect(); signingOut = false }
                }.disabled(signingOut)
            }
        }
        #if os(tvOS)
        .frame(maxWidth: 900)
        #endif
        #if os(iOS)
        .scrollContentBackground(.hidden)
        #endif
        .background(KinoTheme.background)
        .navigationTitle("Settings")
    }
}
