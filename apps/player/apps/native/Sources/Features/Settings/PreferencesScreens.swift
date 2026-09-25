import SwiftUI

struct PlaybackPreferencesScreen: View {
    var itemID: String? = nil
    @Environment(AppSession.self) private var session
    @State private var preferences = PlaybackPreferences()
    @State private var original = PlaybackPreferences()
    @State private var loaded = false
    @State private var busy = false
    @State private var overridden = false
    @State private var message: String?

    var body: some View {
        Form {
            if loaded {
                Section("Playback") {
                    Picker("Speed", selection: $preferences.rate) {
                        ForEach(Array(Set([0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3, preferences.rate])).sorted(), id: \.self) { Text("\($0.formatted())×").tag($0) }
                    }
                    LanguagePicker(title: "Audio language", selection: $preferences.audioLanguage)
                    LanguagePicker(title: "Subtitles", selection: $preferences.subtitleLanguage, allowsOff: true)
                }
                Section {
                    Toggle(isOn: $preferences.dialogueBoost) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Boost Dialog")
                            Text("Brings speech frequencies forward, so quiet conversations are easier to follow without turning everything else up.")
                                .font(.footnote).foregroundStyle(KinoTheme.muted)
                        }
                    }
                    Toggle(isOn: $preferences.nightMode) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Normalize Loudness")
                            Text("Keeps audio at a comfortable, consistent level and takes the edge off very loud scenes.")
                                .font(.footnote).foregroundStyle(KinoTheme.muted)
                        }
                    }
                    if busy { ProgressView("Saving preferences…") }
                } header: {
                    Text("Audio enhancements")
                } footer: {
                    #if os(iOS)
                    Text("Changes save automatically and use a compatible Server stream. Downloads keep the audio already in the file.")
                    #else
                    Text("Uses a compatible Server stream. Downloads keep the audio already in the file.")
                    #endif
                }
                if itemID != nil {
                    Section {
                        Text(overridden ? "This title has its own playback preferences." : "This title follows your Viewer Profile’s defaults.").foregroundStyle(KinoTheme.muted)
                        Button("Use profile defaults") { reset() }.disabled(busy || !overridden)
                    }
                }
                Section { Button(busy ? "Saving…" : "Save preferences") { save() }.disabled(busy || preferences == original) }
            } else if message == nil { ProgressView("Loading preferences…") }
            if let message { Section { Text(message).foregroundStyle(KinoTheme.muted); if !loaded { Button("Try again") { Task { await load() } } } } }
        }
        #if os(tvOS)
        .disabled(busy)
        #endif
        .navigationTitle(itemID == nil ? "Playback preferences" : "This title’s preferences")
        .tvOSConfigurationLayout(title: itemID == nil ? "Playback preferences" : "This title’s preferences", symbol: "play.circle")
        .task(id: "\(session.profileKey ?? ""):\(itemID ?? "defaults")") { await load() }
        #if os(iOS)
        .onChange(of: preferences.dialogueBoost) { _, _ in if loaded && preferences.dialogueBoost != original.dialogueBoost { save() } }
        .onChange(of: preferences.nightMode) { _, _ in if loaded && preferences.nightMode != original.nightMode { save() } }
        #endif
    }
    private func load() async {
        guard let client = session.client else { return }
        message = nil
        do {
            if let itemID { let value = try await client.playbackPreferences(itemID: itemID); try Task.checkCancellation(); preferences = value.playback; overridden = value.overridden }
            else { let value = try await client.mediaPreferences(); try Task.checkCancellation(); preferences = value.playback }
            original = preferences; loaded = true; message = nil
        } catch is CancellationError {} catch { message = AppSession.message(error) }
    }
    private func save() {
        guard let client = session.client, !busy else { return }
        busy = true
        let edited = preferences, baseline = original
        Task {
            defer {
                busy = false
                if preferences.dialogueBoost != edited.dialogueBoost || preferences.nightMode != edited.nightMode { save() }
            }
            do {
                if let itemID {
                    let saved = try await client.savePlaybackPreferences(itemID: itemID, preferences: edited)
                    original = saved.playback; overridden = saved.overridden
                    if preferences == edited { preferences = saved.playback }
                    message = "Preferences saved."
                    if preferences.dialogueBoost == edited.dialogueBoost,
                       preferences.nightMode == edited.nightMode, session.player.currentItem?.id == itemID {
                        do { try await session.player.applyPreferences(saved.playback) }
                        catch { message = "Preferences saved on your Server, but current playback could not update. \(AppSession.message(error))" }
                    }
                } else {
                    var value = try await client.mediaPreferences()
                    if edited.rate != baseline.rate { value.playback.rate = edited.rate }
                    if edited.audioLanguage != baseline.audioLanguage { value.playback.audioLanguage = edited.audioLanguage; value.playback.audioTrack = "" }
                    if edited.subtitleLanguage != baseline.subtitleLanguage { value.playback.subtitleLanguage = edited.subtitleLanguage; value.playback.subtitleTrack = "" }
                    if edited.dialogueBoost != baseline.dialogueBoost { value.playback.dialogueBoost = edited.dialogueBoost }
                    if edited.nightMode != baseline.nightMode { value.playback.nightMode = edited.nightMode }
                    let saved = try await client.saveMediaPreferences(value)
                    original = saved.playback
                    if preferences == edited { preferences = saved.playback }
                    message = "Preferences saved."
                    #if os(iOS)
                    do { try await session.downloads.updatePreferences(saved) }
                    catch { message = "Preferences saved on your Server, but downloads could not update. \(AppSession.message(error))" }
                    #endif
                    if preferences.dialogueBoost == edited.dialogueBoost,
                       preferences.nightMode == edited.nightMode, let item = session.player.currentItem {
                        do {
                            let current = try await client.playbackPreferences(itemID: item.id)
                            if !current.overridden { try await session.player.applyPreferences(current.playback) }
                        } catch { message = "Preferences saved on your Server, but current playback could not update. \(AppSession.message(error))" }
                    }
                }
            } catch { message = AppSession.message(error) }
        }
    }
    private func reset() {
        guard let itemID, let client = session.client, !busy else { return }
        busy = true
        Task {
            defer { busy = false }
            do {
                let saved = try await client.resetPlaybackPreferences(itemID: itemID)
                preferences = saved.playback; original = preferences; overridden = false; message = "Using your profile defaults."
                if session.player.currentItem?.id == itemID { try await session.player.applyPreferences(saved.playback) }
            } catch { message = AppSession.message(error) }
        }
    }
}

private struct LanguagePicker: View {
    let title: String
    @Binding var selection: String
    var allowsOff = false
    private var choices: [String] { Array(Set(["auto", "en", "es", "fr", "de", "it", "pt", "ja", "ko", "zh", selection] + (allowsOff ? ["off"] : []))).sorted() }
    var body: some View {
        Picker(title, selection: $selection) {
            ForEach(choices, id: \.self) { code in Text(code == "auto" ? "Automatic" : code == "off" ? "Off" : Locale.current.localizedString(forLanguageCode: code) ?? code).tag(code) }
        }
    }
}

struct OfflinePreferencesScreen: View {
    @Environment(AppSession.self) private var session
    @State private var preferences = MediaPreferences()
    @State private var original = MediaPreferences()
    @State private var loaded = false
    @State private var busy = false
    @State private var message: String?
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
                    Button(busy ? "Saving…" : "Save preferences") { save() }.disabled(busy || preferences == original)
                    #if os(iOS)
                    Button("Remove this profile’s downloads", role: .destructive) { clears = true }.disabled(busy || session.downloads.downloads.isEmpty)
                    #endif
                }
            } else if message == nil { ProgressView("Loading preferences…") }
            if let message {
                Section {
                    Text(message).foregroundStyle(KinoTheme.muted)
                    if !loaded { Button("Try again") { Task { await load() } } }
                }
            }
        }
        .disabled(busy)
        .navigationTitle("Download preferences")
        .tvOSConfigurationLayout(title: "Download preferences", symbol: "arrow.down.circle")
        .task { await load() }
        .alert("Remove this profile’s downloads?", isPresented: $clears) {
            #if os(iOS)
            Button("Remove downloads", role: .destructive) { Task { do { try await session.downloads.clear(); message = "Downloads removed." } catch { message = AppSession.message(error) } } }
            #endif
            Button("Cancel", role: .cancel) {}
        } message: { Text("This removes saved media for the current Viewer Profile from this device.") }
    }
    private func load() async {
        guard let client = session.client else { return }
        message = nil
        do { let saved = try await client.mediaPreferences(); try Task.checkCancellation(); preferences = saved; original = saved; loaded = true; message = nil }
        catch is CancellationError {} catch { message = AppSession.message(error) }
    }
    private func save() {
        guard let client = session.client, !busy else { return }
        busy = true
        let edited = preferences
        Task {
            defer { busy = false }
            do {
                var value = try await client.mediaPreferences()
                value.wifiOnly = edited.wifiOnly; value.downloadLimitGiB = edited.downloadLimitGiB
                value.autoDownloadNext = edited.autoDownloadNext; value.removeWatched = edited.removeWatched
                let saved = try await client.saveMediaPreferences(value)
                preferences = saved; original = saved
                #if os(iOS)
                try await session.downloads.updatePreferences(saved)
                #endif
                message = "Preferences saved. New transfers use these settings."
            } catch { message = AppSession.message(error) }
        }
    }
}

struct ReaderPreferencesScreen: View {
    @Environment(AppSession.self) private var session
    @State private var preferences = MediaPreferences()
    @State private var original = MediaPreferences()
    @State private var loaded = false
    @State private var busy = false
    @State private var message: String?
    var body: some View {
        Form {
            if loaded {
                Section("Reading") {
                    Picker("Text size", selection: $preferences.readerFontSize) { ForEach(16...32, id: \.self) { Text("\($0) pt").tag($0) } }
                    Picker("Appearance", selection: $preferences.readerTheme) { ForEach(ReaderTheme.allCases, id: \.self) { Text($0.rawValue == "auto" ? "System" : $0.rawValue.capitalized).tag($0) } }
                    Text("Text size changes apply to EPUB books with adjustable layouts. PDF and comic pages keep their original layout.").foregroundStyle(KinoTheme.muted)
                }
                Button(busy ? "Saving…" : "Save preferences") { save() }.disabled(busy || preferences == original)
            } else if message == nil { ProgressView("Loading preferences…") }
            if let message {
                Section {
                    Text(message).foregroundStyle(KinoTheme.muted)
                    if !loaded { Button("Try again") { Task { await load() } } }
                }
            }
        }
        .disabled(busy)
        .navigationTitle("Reader preferences")
        .tvOSConfigurationLayout(title: "Reader preferences", symbol: "book")
        .task { await load() }
    }
    private func load() async {
        guard let client = session.client else { return }
        message = nil
        do {
            let saved = try await client.mediaPreferences()
            try Task.checkCancellation()
            preferences = saved; original = saved; loaded = true
        } catch is CancellationError {} catch { message = AppSession.message(error) }
    }
    private func save() {
        guard let client = session.client, !busy else { return }
        busy = true
        let edited = preferences
        Task {
            defer { busy = false }
            do {
                var value = try await client.mediaPreferences()
                value.readerFontSize = edited.readerFontSize; value.readerTheme = edited.readerTheme
                let saved = try await client.saveMediaPreferences(value)
                preferences = saved; original = saved; message = "Preferences saved."
                session.contentRevision = UUID()
            } catch { message = AppSession.message(error) }
        }
    }
}
