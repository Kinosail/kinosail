import SwiftUI

struct SettingsScreen: View {
    @Environment(AppSession.self) private var session
    @State private var signingOut = false
    @State private var confirmsSignOut = false
    #if os(tvOS)
    @AppStorage(TopShelfPreferences.enabledKey) private var topShelf = true
    #endif
    var body: some View {
        Form {
            Section("Server & Viewer Profile") {
                if let viewer = session.viewer {
                    Label {
                        VStack(alignment: .leading, spacing: 4) {
                            Text(viewer.name).font(.headline).foregroundStyle(KinoTheme.text)
                            Text(viewer.serverName).font(.subheadline).foregroundStyle(KinoTheme.muted)
                        }
                    } icon: {
                        Image(systemName: "person.crop.circle").font(.title).foregroundStyle(KinoTheme.signal)
                    }
                    .padding(.vertical, 8)
                    .accessibilityElement(children: .combine)
                    .accessibilityLabel("Viewer Profile: \(viewer.name). Server: \(viewer.serverName)")
                }
                Button("Change Server", systemImage: "network") { session.showsSetup = true }
                #if os(tvOS)
                Text("Everyone using this Apple TV shares the connected Viewer Profile, including its library access and watch history.")
                    .font(.footnote).foregroundStyle(.secondary)
                #endif
                NavigationLink(value: ScreenDestination.approval) { Label("Connect a TV", systemImage: "tv") }
                if let client = session.client {
                    Text(client.server.url.absoluteString).font(.caption).foregroundStyle(KinoTheme.muted)
                        .textSelection(.enabled).lineLimit(nil)
                }
            }
            Section("Playback & library") {
                NavigationLink(value: ScreenDestination.playbackPreferences) { Label("Playback", systemImage: "play.circle") }
                #if os(iOS)
                NavigationLink(value: ScreenDestination.offlinePreferences) { Label("Downloads", systemImage: "arrow.down.circle") }
                NavigationLink(value: ScreenDestination.readerPreferences) { Label("Reader", systemImage: "book") }
                #endif
                NavigationLink(value: ScreenDestination.progressSync) { Label("Progress sync", systemImage: "arrow.triangle.2.circlepath") }
            }
            Section("Personalize") {
                #if os(iOS)
                NavigationLink(value: ScreenDestination.tabPreferences) { Label("Customize tabs", systemImage: "rectangle.3.group") }
                #endif
                #if os(tvOS)
                Toggle("Show titles on Apple TV Home", isOn: $topShelf)
                Text("Show Continue watching, My List and recent titles when Kinosail is selected in the top row. Titles and artwork are visible to anyone using this Apple TV.")
                    .font(.footnote).foregroundStyle(.secondary)
                #endif
                NavigationLink { SupporterScreen() } label: { Label("Supporter collection", systemImage: "sparkles") }
            }
            Section("About") {
                NavigationLink { ThanksScreen() } label: { Label("Made possible by", systemImage: "heart") }
                PrivacyPolicyLink()
            }
            Section {
                Text("Kinosail plays directly from your Server. Your session is stored securely on this device.")
                    .font(.footnote).foregroundStyle(.secondary)
                Button(signingOut ? "Signing out…" : "Sign out", role: .destructive) {
                    confirmsSignOut = true
                }.disabled(signingOut)
            }
        }
        #if os(iOS)
        .scrollContentBackground(.hidden)
        #endif
        .background(KinoTheme.background)
        .configurationNavigationTitle("Settings", large: true)
        .task(id: session.client?.identity) { _ = try? await session.client?.mediaPreferences(policy: .automatic) }
        .tvOSConfigurationLayout(title: "Settings", symbol: "gearshape")
        .alert("Sign out of this device?", isPresented: $confirmsSignOut) {
            Button("Sign out", role: .destructive) {
                signingOut = true
                Task { await session.disconnect(); signingOut = false }
            }
            Button("Cancel", role: .cancel) {}
        } message: { Text("You’ll need to connect this device to your Server again.") }
    }
}

private struct ThanksScreen: View {
    private let notices = URL(string: "https://github.com/Kinosail/kinosail/blob/main/apps/player/THIRD_PARTY_NOTICES.md")!

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                Text("Thank you to the people who make private playback possible.")
                    .font(.title2.bold())
                Text("FFmpeg and the Jellyfin FFmpeg maintainers give your Server its media tools. hls.js, htmx, jsQR, Manrope, and many Go contributors help Kinosail work across devices.")
                HStack(alignment: .center, spacing: 16) {
                    Image("tmdb-logo").resizable().scaledToFit().frame(width: 80, height: 58)
                        .accessibilityLabel("TMDB")
                    Text("This product uses the TMDB API but is not endorsed or certified by TMDB.")
                        .font(.footnote)
                }
                Text("TMDB logo by Travis Bell · CC BY-SA 4.0. Source and license are in the third-party notices.")
                    .font(.footnote).foregroundStyle(.secondary)
                Text("The Server image contains its third-party notices and license files at /licenses. These Apple apps use Apple's system frameworks and connect to your Server.")
                    .foregroundStyle(.secondary)
                #if os(tvOS)
                QRCodeView(value: notices.absoluteString)
                    .frame(width: 224, height: 224)
                    .accessibilityLabel("Scan to read third-party notices")
                Text(notices.absoluteString).font(.footnote).foregroundStyle(.secondary)
                #else
                Link("View third-party notices", destination: notices)
                #endif
            }
            .frame(maxWidth: 700, alignment: .leading)
            .frame(maxWidth: .infinity)
            .padding(KinoTheme.contentPadding)
        }
        .navigationTitle("Made possible by")
    }
}
