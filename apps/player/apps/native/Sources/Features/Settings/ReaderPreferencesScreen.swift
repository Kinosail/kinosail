import SwiftUI

struct ReaderPreferencesScreen: View {
    @Environment(AppSession.self) private var session
    @State private var preferences = MediaPreferences()
    @State private var original = MediaPreferences()
    @State private var loaded = false
    @State private var busy = false
    @State private var message: String?
    @State private var failed = false
    var body: some View {
        Form {
            if loaded {
                Section {
                    Picker("Text size", selection: $preferences.readerFontSize) { ForEach(16...32, id: \.self) { Text("\($0) pt").tag($0) } }
                    Picker("Appearance", selection: $preferences.readerTheme) { ForEach(ReaderTheme.allCases, id: \.self) { Text($0.rawValue == "auto" ? "System" : $0.rawValue.capitalized).tag($0) } }
                } header: { Text("Reading") } footer: {
                    #if os(iOS)
                    Text("Changes save automatically. Text size applies to EPUB books with adjustable layouts. PDF and comic pages keep their original layout.")
                    #else
                    Text("Text size applies to EPUB books with adjustable layouts. PDF and comic pages keep their original layout.")
                    #endif
                }
                #if os(tvOS)
                Button(busy ? "Saving…" : "Save preferences") { save() }.disabled(busy || preferences == original)
                #endif
            } else if message == nil {
                Section("Reading") {
                    SkeletonRow(kind: .form, status: "Loading preferences…")
                    SkeletonRow(kind: .form)
                    SkeletonRow(kind: .text)
                }
            }
            PreferenceFeedback(busy: busy, message: message, offersRetry: failed) { if loaded { save() } else { Task { await load() } } }
        }
        #if os(tvOS)
        .disabled(busy)
        #endif
        .configurationNavigationTitle("Reader preferences")
        .tvOSConfigurationLayout(title: "Reader preferences", symbol: "book")
        .task(id: session.client?.identity) { await load() }
        #if os(iOS)
        .onChange(of: preferences) { _, _ in if loaded && preferences != original { save() } }
        #endif
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
                value.readerFontSize = edited.readerFontSize; value.readerTheme = edited.readerTheme
                let saved = try await client.saveMediaPreferences(value)
                guard session.client?.identity == client.identity else { return }
                original = saved
                if preferences == edited { preferences = saved }
                session.contentRevision = UUID()
            } catch { message = "Changes weren’t saved. \(AppSession.message(error))"; failed = true }
        }
    }
}
