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
            if session.usesDeviceLibrary { LibrariesSection() } else {
            Section {
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
                NavigationLink(value: ScreenDestination.approval) { Label("Connect a TV", systemImage: "tv") }
                if let client = session.client {
                    Text(client.server.url.absoluteString).font(.caption).foregroundStyle(KinoTheme.muted)
                        .lineLimit(nil)
                        #if os(iOS)
                        .textSelection(.enabled)
                        #endif
                }
            } header: {
                Text("Server & Viewer Profile")
            } footer: {
                #if os(tvOS)
                Text("Everyone using this Apple TV shares the connected Viewer Profile, including its library access and watch history.")
                    .font(.footnote).foregroundStyle(KinoTheme.muted)
                #endif
            }
            }
            Section("Playback & library") {
                NavigationLink(value: ScreenDestination.playbackPreferences) { Label("Playback", systemImage: "play.circle") }
                #if os(iOS)
                if !session.usesDeviceLibrary {
                    NavigationLink(value: ScreenDestination.offlinePreferences) { Label("Downloads", systemImage: "arrow.down.circle") }
                    NavigationLink(value: ScreenDestination.readerPreferences) { Label("Reader", systemImage: "book") }
                }
                #endif
            }
            #if os(tvOS)
            Section {
                Toggle("Show titles on Apple TV Home", isOn: $topShelf)
            } header: {
                Text("Personalize")
            } footer: {
                Text("Show Continue watching, My List and recent titles when Kinosail is selected in the top row. Titles and artwork are visible to anyone using this Apple TV.")
                    .font(.footnote).foregroundStyle(KinoTheme.muted)
            }
            #endif
            Section {
                #if os(iOS)
                NavigationLink(value: ScreenDestination.tabPreferences) { Label("Customize tabs", systemImage: "rectangle.3.group") }
                #endif
                if !session.usesDeviceLibrary {
                    NavigationLink { SupporterScreen() } label: { Label("Supporter collection", systemImage: "sparkles") }
                }
            } header: {
                #if os(iOS)
                Text("Personalize")
                #endif
            }
            Section("About") {
                NavigationLink { ThanksScreen() } label: { Label("Made possible by", systemImage: "heart") }
                PrivacyPolicyLink()
            }
            Section {
                #if os(iOS)
                Text(sessionNote).font(.footnote).foregroundStyle(.secondary)
                #endif
                Button(signingOut ? "Signing out…" : "Sign out", role: .destructive) {
                    confirmsSignOut = true
                }.disabled(signingOut)
            } footer: {
                #if os(tvOS)
                Text(sessionNote)
                    .font(.footnote).foregroundStyle(KinoTheme.muted)
                #endif
            }
        }
        #if os(iOS)
        .scrollContentBackground(.hidden)
        #else
        .background(KinoTheme.background)
        #endif
        .configurationNavigationTitle("Settings", large: true)
        .task(id: session.client?.identity) { _ = try? await session.client?.mediaPreferences(policy: .automatic) }
        .tvOSConfigurationLayout(title: "Settings", symbol: "gearshape")
        .alert("Sign out of this device?", isPresented: $confirmsSignOut) {
            Button("Sign out", role: .destructive) {
                signingOut = true
                Task { await session.disconnect(); signingOut = false }
            }
            Button("Cancel", role: .cancel) {}
        } message: { Text(session.usesDeviceLibrary ? "This removes every library from this device." : "You’ll need to connect this device to your Server again.") }
    }

    private var sessionNote: String {
        session.usesDeviceLibrary ? "Kinosail plays directly from your servers. Sign-ins are stored securely on this device."
            : "Kinosail plays directly from your Server. Your session is stored securely on this device."
    }
}

private struct ThanksScreen: View {
    private let notices = URL(string: "https://github.com/Kinosail/kinosail/blob/main/apps/player/THIRD_PARTY_NOTICES.md")!

    var body: some View {
        ScrollView {
            #if os(tvOS)
            HStack(alignment: .top, spacing: 72) {
                credits.frame(maxWidth: 760, alignment: .leading)
                VStack(spacing: 16) {
                    QRCodeView(value: notices.absoluteString)
                        .frame(width: 224, height: 224)
                        .accessibilityLabel("Scan to read third-party notices")
                    Text("Scan to read the third-party notices.")
                        .font(.footnote).foregroundStyle(.secondary).multilineTextAlignment(.center)
                }
                .frame(width: 240)
            }
            .frame(maxWidth: .infinity)
            .padding(KinoTheme.contentPadding)
            #else
            VStack(alignment: .leading, spacing: 20) {
                credits
                Link("View third-party notices", destination: notices)
            }
            .frame(maxWidth: 700, alignment: .leading)
            .frame(maxWidth: .infinity)
            .padding(KinoTheme.contentPadding)
            #endif
        }
        .navigationTitle("Made possible by")
    }

    private var credits: some View {
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
        }
    }
}
