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
    @State private var failed = false
    @State private var resetting = false

    var body: some View {
        Form {
            if loaded {
                Section("Playback") {
                    #if os(tvOS)
                    TVPreferenceChoiceLink(title: "Speed", symbol: "speedometer", selection: $preferences.rate,
                                           choices: PlaybackPreferenceChoices.rates(including: preferences.rate), label: PlaybackPreferenceChoices.rateLabel)
                    TVPreferenceChoiceLink(title: "Audio language", symbol: "speaker.wave.2", selection: $preferences.audioLanguage,
                                           choices: PlaybackPreferenceChoices.languages(including: preferences.audioLanguage), label: PlaybackPreferenceChoices.languageLabel)
                    TVPreferenceChoiceLink(title: "Subtitles", symbol: "captions.bubble", selection: $preferences.subtitleLanguage,
                                           choices: PlaybackPreferenceChoices.languages(including: preferences.subtitleLanguage, allowsOff: true),
                                           label: PlaybackPreferenceChoices.languageLabel)
                    #else
                    Picker("Speed", selection: $preferences.rate) {
                        ForEach(PlaybackPreferenceChoices.rates(including: preferences.rate), id: \.self) { Text(PlaybackPreferenceChoices.rateLabel($0)).tag($0) }
                    }
                    LanguagePicker(title: "Audio language", selection: $preferences.audioLanguage)
                    LanguagePicker(title: "Subtitles", selection: $preferences.subtitleLanguage, allowsOff: true)
                    #endif
                }
                Section {
                    Toggle(isOn: $preferences.dialogueBoost) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Boost Dialog")
                            Text("Makes quiet speech easier to hear.").font(.footnote).foregroundStyle(KinoTheme.muted)
                        }
                    }
                    Toggle(isOn: $preferences.nightMode) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Normalize Loudness")
                            Text("Evens out the volume and softens very loud scenes.").font(.footnote).foregroundStyle(KinoTheme.muted)
                        }
                    }
                } header: {
                    Text("Audio enhancements")
                } footer: {
                    Text(footer)
                }
                if itemID != nil, overridden {
                    Section {
                        Button("Use profile defaults") { reset() }.disabled(busy)
                    } footer: {
                        Text("This title has its own preferences. Other titles use your Viewer Profile’s defaults.")
                    }
                }
            } else if message == nil {
                Section("Playback") {
                    ForEach(0..<3) { index in SkeletonRow(kind: .form, status: index == 0 ? "Loading preferences…" : nil) }
                }
                Section("Audio enhancements") {
                    ForEach(0..<2) { _ in SkeletonRow(kind: .form) }
                }
            }
            PreferenceFeedback(busy: busy, message: message, offersRetry: failed) { if loaded { save() } else { Task { await load() } } }
        }
        .disabled(resetting)
        .configurationNavigationTitle(itemID == nil ? "Playback preferences" : "This title’s preferences")
        .tvOSConfigurationLayout(title: itemID == nil ? "Playback preferences" : "This title’s preferences", symbol: "play.circle")
        .task(id: "\(session.profileKey ?? ""):\(itemID ?? "defaults")") { await load() }
        .onChange(of: preferences) { _, _ in if loaded && preferences != original { save() } }
    }
    private var footer: String {
        let scope = itemID == nil ? "Changes save automatically." : "Changes save automatically and apply only to this title."
        #if os(iOS)
        return scope + " Audio enhancements use a compatible Server stream. Downloads keep the audio already in the file."
        #else
        return scope + " Audio enhancements use a compatible Server stream."
        #endif
    }
    private func load() async {
        message = nil; failed = false
        guard let client = session.client else { message = AppSession.message(ClientError.unavailable); failed = true; return }
        do {
            if !loaded {
                if let itemID, let value = try? await client.playbackPreferences(itemID: itemID, policy: .cached) {
                    preferences = value.playback; original = preferences; overridden = value.overridden; loaded = true
                } else if itemID == nil, let value = try? await client.mediaPreferences(policy: .cached) {
                    preferences = value.playback; original = preferences; loaded = true
                }
            }
            let next: PlaybackPreferences
            if let itemID { let value = try await client.playbackPreferences(itemID: itemID, policy: .automatic); next = value.playback; overridden = value.overridden }
            else { next = try await client.mediaPreferences(policy: .automatic).playback }
            try Task.checkCancellation()
            guard session.client?.identity == client.identity else { return }
            if preferences == original { preferences = next; original = next }
            loaded = true
        } catch is CancellationError {} catch { message = AppSession.message(error); failed = true }
    }
    private func save() {
        guard let client = session.client, !busy else { return }
        busy = true; message = nil; failed = false
        let edited = preferences, baseline = original
        var withoutEffects = edited
        withoutEffects.dialogueBoost = baseline.dialogueBoost; withoutEffects.nightMode = baseline.nightMode
        let effectsOnly = withoutEffects == baseline
        Task {
            defer {
                busy = false
                if preferences != edited && preferences != original { save() }
            }
            do {
                if let itemID {
                    let saved = try await client.savePlaybackPreferences(itemID: itemID, preferences: edited)
                    guard session.client?.identity == client.identity else { return }
                    original = saved.playback; overridden = saved.overridden
                    if preferences == edited { preferences = saved.playback }
                    if preferences == original, session.player.currentItem?.id == itemID {
                        do { try await session.player.applyPreferences(saved.playback, preserveDeviceChoices: effectsOnly) }
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
                    guard session.client?.identity == client.identity else { return }
                    original = saved.playback
                    if preferences == edited { preferences = saved.playback }
                    #if os(iOS)
                    do { try await session.downloads.updatePreferences(saved) }
                    catch { message = "Preferences saved on your Server, but downloads could not update. \(AppSession.message(error))" }
                    #endif
                    if preferences == original, let item = session.player.currentItem {
                        do {
                            let current = try await client.playbackPreferences(itemID: item.id)
                            if !current.overridden { try await session.player.applyPreferences(current.playback, preserveDeviceChoices: effectsOnly) }
                        } catch { message = "Preferences saved on your Server, but current playback could not update. \(AppSession.message(error))" }
                    }
                }
            } catch { message = "Changes weren’t saved. \(AppSession.message(error))"; failed = true }
        }
    }
    private func reset() {
        guard let itemID, let client = session.client, !busy else { return }
        busy = true; resetting = true; failed = false; message = nil
        Task {
            defer { busy = false; resetting = false }
            do {
                let saved = try await client.resetPlaybackPreferences(itemID: itemID)
                guard session.client?.identity == client.identity else { return }
                preferences = saved.playback; original = preferences; overridden = false; message = "Using your profile defaults."
                if session.player.currentItem?.id == itemID { try await session.player.applyPreferences(saved.playback) }
            } catch { message = AppSession.message(error) }
        }
    }
}

enum PlaybackPreferenceChoices {
    static func rates(including rate: Double) -> [Double] { Array(Set([0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3, rate])).sorted() }
    static func rateLabel(_ rate: Double) -> String { "\(rate.formatted())×" }
    static func languages(including selection: String, allowsOff: Bool = false) -> [String] {
        Array(Set(["auto", "en", "es", "fr", "de", "it", "pt", "ja", "ko", "zh", selection] + (allowsOff ? ["off"] : []))).sorted()
    }
    static func languageLabel(_ code: String) -> String {
        code == "auto" ? "Automatic" : code == "off" ? "Off" : Locale.current.localizedString(forLanguageCode: code) ?? code
    }
}

private struct LanguagePicker: View {
    let title: String
    @Binding var selection: String
    var allowsOff = false
    var body: some View {
        Picker(title, selection: $selection) {
            ForEach(PlaybackPreferenceChoices.languages(including: selection, allowsOff: allowsOff), id: \.self) {
                Text(PlaybackPreferenceChoices.languageLabel($0)).tag($0)
            }
        }
    }
}

#if os(tvOS)
private struct TVPreferenceChoiceLink<Value: Hashable & LosslessStringConvertible>: View {
    let title: String
    let symbol: String
    @Binding var selection: Value
    let choices: [Value]
    let label: (Value) -> String

    var body: some View {
        NavigationLink {
            TVPlaybackChoiceScreen(title: title, symbol: symbol,
                                   choices: choices.map { TVPlaybackChoice(id: $0.description, title: label($0), selected: $0 == selection) }) { id in
                guard let value = choices.first(where: { $0.description == id }) else { throw ClientError.invalidInput("Choose one of the listed options.") }
                selection = value
            }
        } label: {
            TVPlaybackValueLabel(title: title, value: label(selection))
        }
    }
}
#endif
