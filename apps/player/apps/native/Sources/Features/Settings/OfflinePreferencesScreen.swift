import SwiftUI

struct OfflinePreferencesScreen: View {
    @Environment(AppSession.self) private var session
    @State private var preferences = MediaPreferences()
    @State private var original = MediaPreferences()
    @State private var loaded = false
    @State private var busy = false
    @State private var message: String?
    @State private var failed = false
    @State private var clears = false

    var body: some View {
        Form {
            if loaded {
                Section("Connection & storage") {
                    Toggle("Wi-Fi only", isOn: $preferences.wifiOnly)
                    Picker("Storage limit", selection: $preferences.downloadLimitGiB) {
                        ForEach(Array(Set([0, 5, 10, 20, 50, 100, 200, preferences.downloadLimitGiB])).sorted(), id: \.self) { value in Text(value == 0 ? "No limit" : "\(value) GB").tag(value) }
                    }
                }
                Section("Episodes") {
                    Picker("Automatically download next", selection: $preferences.autoDownloadNext) {
                        ForEach(0...3, id: \.self) { count in Text(count == 0 ? "Off" : "\(count) episode\(count == 1 ? "" : "s")").tag(count) }
                    }
                    Toggle("Remove watched downloads", isOn: $preferences.removeWatched)
                }
                Section {
                    #if os(tvOS)
                    Button(busy ? "Saving…" : "Save preferences") { save() }.disabled(busy || preferences == original)
                    #endif
                    #if os(iOS)
                    Button("Remove this profile’s downloads", role: .destructive) { clears = true }.disabled(busy || session.downloads.downloads.isEmpty)
                    #endif
                } footer: {
                    #if os(iOS)
                    Text("Changes save automatically. New transfers use these settings.")
                    #else
                    Text("New transfers use these settings.")
                    #endif
                }
            } else if message == nil {
                Section("Connection & storage") {
                    ForEach(0..<2) { index in SkeletonRow(kind: .form, status: index == 0 ? "Loading preferences…" : nil) }
                }
                Section("Episodes") {
                    ForEach(0..<2) { _ in SkeletonRow(kind: .form) }
                }
            }
            PreferenceFeedback(busy: busy, message: message, offersRetry: failed) { if loaded { save() } else { Task { await load() } } }
        }
        #if os(tvOS)
        .disabled(busy)
        #endif
        .configurationNavigationTitle("Download preferences")
        .tvOSConfigurationLayout(title: "Download preferences", symbol: "arrow.down.circle")
        .task(id: session.client?.identity) { await load() }
        #if os(iOS)
        .onChange(of: preferences) { _, _ in if loaded && preferences != original { save() } }
        #endif
        .alert("Remove this profile’s downloads?", isPresented: $clears) {
            #if os(iOS)
            Button("Remove downloads", role: .destructive) { Task { do { try await session.downloads.clear(); message = "Downloads removed." } catch { message = AppSession.message(error) } } }
            #endif
            Button("Cancel", role: .cancel) {}
        } message: { Text("This removes saved media for the current Viewer Profile from this device.") }
    }
    private func load() async {
        message = nil; failed = false
        guard let client = session.client else { message = AppSession.message(ClientError.unavailable); failed = true; return }
        do {
            if !loaded, let saved = try? await client.mediaPreferences(policy: .cached) { preferences = saved; original = saved; loaded = true }
            let saved = try await client.mediaPreferences(policy: .automatic)
            try Task.checkCancellation()
            guard session.client?.identity == client.identity else { return }
            if preferences == original { preferences = saved; original = saved }
            loaded = true
        } catch is CancellationError {} catch { message = AppSession.message(error); failed = true }
    }
    private func save() {
        guard let client = session.client, !busy else { return }
        busy = true; message = nil; failed = false
        let edited = preferences
        Task {
            defer { busy = false; if preferences != edited && preferences != original { save() } }
            do {
                var value = try await client.mediaPreferences()
                value.wifiOnly = edited.wifiOnly; value.downloadLimitGiB = edited.downloadLimitGiB
                value.autoDownloadNext = edited.autoDownloadNext; value.removeWatched = edited.removeWatched
                let saved = try await client.saveMediaPreferences(value)
                guard session.client?.identity == client.identity else { return }
                original = saved
                if preferences == edited { preferences = saved }
                #if os(iOS)
                try await session.downloads.updatePreferences(saved)
                #endif
            } catch { message = "Changes weren’t saved. \(AppSession.message(error))"; failed = true }
        }
    }
}
