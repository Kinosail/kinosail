import SwiftUI

struct DownloadOptionsScreen: View {
    let item: MediaItem
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var quality = DownloadQuality.compatible
    @State private var options: DownloadTrackOptions?
    @State private var audio = Set<Int>()
    @State private var subtitles = Set<Int>()
    @State private var failure: String?
    @State private var busy = false
    @State private var loaded = false

    init(item: MediaItem) {
        self.item = item
        _quality = State(initialValue: item.isAudio ? .audio : .compatible)
    }

    var body: some View {
        #if os(iOS)
        Form {
            Section {
                HStack(spacing: 14) {
                    Artwork(path: item.isAudio ? item.poster : item.landscapeArtwork, symbol: item.kind.symbol,
                            ratio: item.isAudio ? 1 : 16 / 9, dimension: 800, fillsFrame: true)
                        .frame(width: item.isAudio ? 64 : 112)
                        .background(KinoTheme.surface)
                        .clipShape(.rect(cornerRadius: 10))
                    VStack(alignment: .leading, spacing: 4) {
                        Text(item.title).font(.headline)
                        if !item.subtitle.isEmpty { Text(item.subtitle).font(.caption).foregroundStyle(.secondary) }
                    }
                }
                .accessibilityElement(children: .combine)
            }
            .listRowBackground(Color.clear)
            Section {
                Picker("Quality", selection: $quality) {
                    ForEach(qualities) { value in
                        VStack(alignment: .leading, spacing: 2) {
                            Text(value.title)
                            if !note(for: value).isEmpty { Text(note(for: value)).font(.caption).foregroundStyle(.secondary) }
                        }
                        .tag(value)
                    }
                }
                .pickerStyle(.inline)
                .labelsHidden()
                .sensoryFeedback(.selection, trigger: quality)
            } header: { Text("Quality") } footer: {
                if quality == .original { Text("Original includes all audio and subtitle tracks in the file. This device must support the file’s format.") }
                else { Text("Choose which audio and subtitle tracks to include in the compatible copy. The original file stays on your Server.") }
            }
            if quality != .original, let options {
                Section("Audio") {
                    ForEach(options.audio) { track in Toggle(track.label, isOn: selected(track.index, in: $audio)) }
                }
                if !options.subtitles.isEmpty {
                    Section("Subtitles") {
                        ForEach(options.subtitles) { track in Toggle(track.label, isOn: selected(track.index, in: $subtitles)) }
                        Text("Styled or image subtitles may require a format this device cannot play offline. Every download is checked before it becomes ready.").font(.caption).foregroundStyle(.secondary)
                    }
                }
            }
            if !loaded && options == nil && quality != .original {
                Section("Audio") {
                    ForEach(0..<2) { index in SkeletonRow(kind: .form, status: index == 0 ? "Loading download options…" : nil) }
                }
            }
            if !loaded && quality == .original { Text("Checking download options…").foregroundStyle(KinoTheme.muted) }
            if let failure {
                Text(failure).foregroundStyle(.secondary)
                if item.kind == .video && options == nil {
                    Button("Try loading tracks again") { Task { await loadTracks() } }.disabled(!loaded)
                }
            }
        }
        .safeAreaInset(edge: .bottom, spacing: 0) {
            VStack(spacing: 8) {
                Button { enqueue() } label: {
                    Label(busy ? "Adding download…" : "Add to Downloads", systemImage: "arrow.down")
                        .frame(maxWidth: .infinity)
                }
                .disabled(busy || !loaded || quality != .original && item.kind == .video && options == nil)
                .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).controlSize(.large)
                .tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                Text("Downloads use your saved network and storage preferences.").font(.caption).foregroundStyle(.secondary)
            }
            .padding(.horizontal, 20).padding(.top, 12).padding(.bottom, 8)
            .background(.bar)
        }
        .configurationNavigationTitle("Download")
        .toolbar { ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() }.disabled(busy) } }
        .task(id: item.id) {
            quality = item.isAudio ? .audio : .compatible
            options = nil; audio = []; subtitles = []; failure = nil; loaded = false
            if item.kind == .video { await loadTracks() }
            else { loaded = true }
        }
        #else
        ContentUnavailableView("Download on iPhone or iPad", systemImage: "iphone")
        #endif
    }
    private func note(for quality: DownloadQuality) -> String {
        switch quality {
        case .compatible: "Keeps the video as-is when this device can play it. The file can be as big as the original."
        case .fullHD, .hd: "Re-encoded on your Server to H.264 at up to \(quality.title). Smaller, but takes longer to prepare."
        case .original: "The exact file from your Server."
        default: ""
        }
    }
    private var qualities: [DownloadQuality] { item.isAudio ? [.audio, .original] : [.compatible, .fullHD, .hd, .original] }
    private func selected(_ index: Int, in values: Binding<Set<Int>>) -> Binding<Bool> {
        Binding(get: { values.wrappedValue.contains(index) }, set: { if $0 { values.wrappedValue.insert(index) } else { values.wrappedValue.remove(index) } })
    }
    #if os(iOS)
    private func loadTracks() async {
        loaded = false
        failure = nil
        guard let client = session.client else { failure = AppSession.message(ClientError.unavailable); loaded = true; return }
        do {
            let tracks = try await client.downloadTracks(itemID: item.id)
            try Task.checkCancellation()
            options = tracks; audio = Set(tracks.audio.map(\.index))
        } catch is CancellationError { return }
        catch { failure = AppSession.message(error) }
        loaded = true
    }

    private func enqueue() {
        guard let client = session.client, !busy else { return }
        busy = true
        let tracks = quality == .original || item.isAudio ? nil : DownloadTrackSelection(audio: audio.sorted(), subtitles: subtitles.sorted())
        Task {
            defer { busy = false }
            do { try await session.downloads.enqueue(item: item, quality: quality, tracks: tracks, client: client); dismiss() }
            catch { failure = AppSession.message(error) }
        }
    }
    #endif
}
