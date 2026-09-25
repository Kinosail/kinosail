import SwiftUI

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
            } else if message == nil {
                Section("Reading") {
                    SkeletonRow(kind: .form, status: "Loading preferences…")
                    SkeletonRow(kind: .form)
                    SkeletonRow(kind: .text)
                }
            }
            if let message {
                Section {
                    Text(message).foregroundStyle(KinoTheme.muted)
                    if !loaded { Button("Try again") { Task { await load() } } }
                }
            }
        }
        .disabled(busy)
        .configurationNavigationTitle("Reader preferences")
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
