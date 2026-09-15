import SwiftUI

struct DownloadsScreen: View {
    @Environment(AppSession.self) private var session
    #if os(iOS)
    @State private var failure: String?
    @State private var busy = false
    @State private var reset = false
    #endif

    var body: some View {
        #if os(iOS)
        List {
            if let message = failure ?? session.downloads.message {
                Section {
                    Text(message).foregroundStyle(.secondary)
                    if session.downloads.message != nil { Button("Reset downloads on this device", role: .destructive) { reset = true } }
                }
            }
            if session.viewer?.downloads != true {
                ContentUnavailableView("Downloads aren’t available", systemImage: "arrow.down.circle", description: Text("Downloads must be enabled for your Viewer Profile on the Server."))
            } else if session.downloads.downloads.isEmpty {
                ContentUnavailableView("Take a title with you", systemImage: "arrow.down.circle", description: Text("Open a movie, episode, or audiobook in your library and choose Download."))
            }
            ForEach(session.downloads.downloads) { download in
                HStack(alignment: .top, spacing: 16) {
                    Artwork(path: download.item.poster, symbol: download.item.kind.symbol, ratio: download.item.isAudio ? 1 : 2 / 3, dimension: 800)
                        .frame(width: 76).clipShape(.rect(cornerRadius: 10))
                    VStack(alignment: .leading, spacing: 10) {
                        Text(download.item.title).font(.headline)
                        Text(download.quality.title).font(.caption).foregroundStyle(.secondary)
                        if download.state == .ready {
                            NavigationLink(value: ScreenDestination.offlinePlayback(download.id)) { Label("Play offline", systemImage: "play.fill") }
                                .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                        } else {
                            Text(download.state == .preparing ? "Preparing on your Server" : download.state.rawValue.capitalized).font(.callout)
                            if download.totalBytes > 0 {
                                ProgressView(value: Double(download.receivedBytes), total: Double(download.totalBytes))
                                    .accessibilityLabel("Download progress")
                                Text("\(bytes(download.receivedBytes)) of \(bytes(download.totalBytes))").font(.caption.monospacedDigit()).foregroundStyle(.secondary)
                            }
                        }
                        if let message = download.message { Text(message).font(.caption).foregroundStyle(.secondary) }
                        HStack {
                            if [.paused, .failed].contains(download.state) {
                                Button("Resume") { perform { try await session.downloads.resume(id: download.id) } }
                            } else if download.state != .ready && download.state != .verifying {
                                Button("Pause") { perform { try await session.downloads.pause(id: download.id) } }
                            }
                            Button("Remove", role: .destructive) { perform { try await session.downloads.remove(id: download.id) } }
                        }.disabled(busy || session.downloads.busy)
                    }
                }.padding(.vertical, 10)
            }
        }
        .navigationTitle("Downloads")
        .toolbar { ToolbarItem(placement: .primaryAction) { NavigationLink(value: ScreenDestination.offlinePreferences) { Label("Download settings", systemImage: "gearshape") } } }
        .alert("Reset downloads?", isPresented: $reset) {
            Button("Remove all downloads", role: .destructive) { perform {
                try await session.downloads.resetDeviceStorage()
                if let client = session.client, let viewer = session.viewer { await session.downloads.restore(viewer: viewer, server: client.server, client: client) }
            } }
            Button("Cancel", role: .cancel) {}
        } message: { Text("This removes downloaded media for every Viewer Profile on this device. Your Server library stays available.") }
        #else
        ContentUnavailableView("Download on iPhone or iPad", systemImage: "iphone", description: Text("Use Kinosail on your iPhone or iPad to save media for offline playback."))
        #endif
    }
    #if os(iOS)
    private func bytes(_ value: Int64) -> String { ByteCountFormatter.string(fromByteCount: value, countStyle: .file) }
    private func perform(_ action: @escaping @MainActor () async throws -> Void) {
        guard !busy else { return }
        busy = true
        Task { defer { busy = false }; do { try await action(); failure = nil } catch { failure = AppSession.message(error) } }
    }
    #endif
}

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

    var body: some View {
        #if os(iOS)
        Form {
            Section(item.title) {
                Picker("Quality", selection: $quality) {
                    ForEach(qualities) { value in Text(value.title).tag(value) }
                }
                if quality == .original { Text("Original includes all audio and subtitle tracks in the file. This device must support the file’s format.").foregroundStyle(.secondary) }
                else { Text("Choose which audio and subtitle tracks to include in the compatible copy. The original file stays on your Server.").foregroundStyle(.secondary) }
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
            if !loaded { Text("Loading download options…").foregroundStyle(.secondary) }
            if let failure { Text(failure).foregroundStyle(.secondary) }
            Section {
                Button(busy ? "Adding download…" : "Add to Downloads") { enqueue() }.disabled(busy || !loaded || quality != .original && item.kind == .video && options == nil)
                Text("Downloads use your saved network and storage preferences.").font(.caption).foregroundStyle(.secondary)
            }
        }
        .navigationTitle("Download")
        .toolbar { ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() }.disabled(busy) } }
        .task(id: item.id) {
            quality = item.isAudio ? .audio : .compatible
            guard let client = session.client else { return }
            if item.kind == .video {
                do { let tracks = try await client.downloadTracks(itemID: item.id); try Task.checkCancellation(); options = tracks; audio = Set(tracks.audio.map(\.index)) }
                catch is CancellationError { return } catch { failure = AppSession.message(error) }
            }
            loaded = true
        }
        #else
        ContentUnavailableView("Download on iPhone or iPad", systemImage: "iphone")
        #endif
    }
    private var qualities: [DownloadQuality] { item.isAudio ? [.audio, .original] : [.compatible, .fullHD, .hd, .original] }
    private func selected(_ index: Int, in values: Binding<Set<Int>>) -> Binding<Bool> {
        Binding(get: { values.wrappedValue.contains(index) }, set: { if $0 { values.wrappedValue.insert(index) } else { values.wrappedValue.remove(index) } })
    }
    #if os(iOS)
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
