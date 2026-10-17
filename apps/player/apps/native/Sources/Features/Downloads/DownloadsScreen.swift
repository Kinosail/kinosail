import SwiftUI

struct DownloadsScreen: View {
    @Environment(AppSession.self) private var session
    @Environment(\.dynamicTypeSize) private var dynamicType
    #if os(iOS)
    @State private var failure: String?
    @State private var busy = false
    @State private var reset = false
    @State private var removal: OfflineDownload?
    @State private var selection = Set<String>()
    @State private var editing = EditMode.inactive
    @State private var bulkRemoval = false
    #endif

    var body: some View {
        #if os(iOS)
        List(selection: $selection) {
            if session.connection.unavailable {
                Section {
                    Text("Completed downloads are ready to play offline. Unfinished downloads need a connection to continue.")
                        .foregroundStyle(KinoTheme.muted)
                }
            }
            if let message = failure ?? session.downloads.message {
                Section {
                    Text(message).foregroundStyle(.secondary)
                    if session.downloads.message != nil { Button("Reset downloads on this device", role: .destructive) { reset = true } }
                }
            }
            if session.downloads.restoring && session.downloads.downloads.isEmpty {
                ForEach(0..<3) { index in SkeletonRow(kind: .media, status: index == 0 ? "Opening your downloads…" : nil) }
            } else if session.downloads.restoring {
                Text("Refreshing downloads…").foregroundStyle(KinoTheme.muted)
            } else if session.viewer?.downloads != true {
                ContentUnavailableView("Downloads aren’t available", systemImage: "arrow.down.circle", description: Text("Downloads must be enabled for your Viewer Profile on the Server."))
            } else if session.downloads.downloads.isEmpty {
                ContentUnavailableView(session.connection.unavailable ? "No downloads on this device" : "Take a title with you", systemImage: "arrow.down.circle",
                    description: Text(session.connection.unavailable ? "When you’re connected again, open a title and choose Download to save it for offline playback." : "Open a movie, episode, or audiobook in your library and choose Download."))
            }
            ForEach(session.downloads.downloads.filter { $0.state == .ready } + session.downloads.downloads.filter { $0.state != .ready }) { download in
                HStack(alignment: .top, spacing: 16) {
                    if !dynamicType.isAccessibilitySize {
                        Artwork(path: download.item.poster, symbol: download.item.kind.symbol, ratio: download.item.isAudio ? 1 : 2 / 3, dimension: 800)
                            .frame(width: 76).clipShape(.rect(cornerRadius: 10))
                    }
                    VStack(alignment: .leading, spacing: 10) {
                        Text(download.item.title).font(.headline)
                        Text(download.quality.title).font(.caption).foregroundStyle(.secondary)
                        if download.state == .ready {
                            NavigationLink(value: ScreenDestination.offlinePlayback(download.id)) { Label("Play offline", systemImage: "play.fill") }
                                .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                        } else {
                            Text(download.state == .preparing ? "Preparing on your Server" : download.state.rawValue.capitalized).font(.callout)
                            if download.state == .preparing || (download.state == .queued && download.totalBytes == 0) {
                                if let percent = download.preparingPercent {
                                    ProgressView(value: Double(percent), total: 100).accessibilityLabel("Preparing download").accessibilityValue("\(percent) percent")
                                } else {
                                    ProgressView().progressViewStyle(.linear).accessibilityLabel("Preparing download")
                                }
                                TimelineView(.periodic(from: .now, by: 1)) { context in
                                    Text(preparingDetail(download, now: context.date)).font(.caption).foregroundStyle(.secondary)
                                }
                            }
                            if download.totalBytes > 0 {
                                ProgressView(value: Double(download.receivedBytes), total: Double(download.totalBytes))
                                    .accessibilityLabel("Download progress")
                                    .accessibilityValue("\(Int(Double(download.receivedBytes) / Double(download.totalBytes) * 100)) percent")
                                Text(transferDetail(download)).font(.caption.monospacedDigit()).foregroundStyle(.secondary)
                            }
                        }
                        if let message = download.message { Text(message).font(.caption).foregroundStyle(.secondary) }
                        HStack {
                            if [.paused, .failed].contains(download.state) {
                                Button("Resume") { perform { try await session.downloads.resume(id: download.id) } }
                                    .frame(minHeight: 44)
                                    .disabled(session.connection.unavailable)
                            } else if download.state != .ready && download.state != .verifying {
                                Button("Pause") { perform { try await session.downloads.pause(id: download.id) } }.frame(minHeight: 44)
                            }
                            Button("Remove", role: .destructive) { removal = download }.frame(minHeight: 44)
                        }.buttonStyle(.borderless).disabled(busy || session.downloads.busy)
                    }
                }.padding(.vertical, 10)
            }
        }
        .navigationTitle("Downloads")
        .scrollContentBackground(.hidden).background(KinoTheme.background)
        .environment(\.editMode, $editing)
        .toolbar {
            ToolbarItem(placement: .primaryAction) { NavigationLink(value: ScreenDestination.offlinePreferences) { Label("Download settings", systemImage: "gearshape") } }
            if !session.downloads.downloads.isEmpty { ToolbarItem(placement: .secondaryAction) { EditButton() } }
            if editing.isEditing {
                ToolbarItemGroup(placement: .bottomBar) {
                    Button(selection.count == session.downloads.downloads.count ? "Select none" : "Select all") {
                        selection = selection.count == session.downloads.downloads.count ? [] : Set(session.downloads.downloads.map(\.id))
                    }
                    Spacer()
                    Button("Remove\(selection.isEmpty ? "" : " \(selection.count)")", role: .destructive) { bulkRemoval = true }
                        .disabled(selection.isEmpty || busy || session.downloads.busy)
                }
            }
        }
        .onChange(of: session.downloads.downloads.map(\.id)) { _, ids in selection.formIntersection(ids) }
        .alert("Remove \(selection.count) downloads?", isPresented: $bulkRemoval) {
            Button("Remove downloads", role: .destructive) {
                let ids = Array(selection)
                perform { for id in ids { try await session.downloads.remove(id: id) }; selection = []; editing = .inactive }
            }
            Button("Cancel", role: .cancel) {}
        } message: { Text("This removes the selected downloads from this device. The titles stay in your Server library.") }
        .alert("Reset downloads?", isPresented: $reset) {
            Button("Remove all downloads", role: .destructive) { perform {
                try await session.downloads.resetDeviceStorage()
                if let client = session.client, let viewer = session.viewer { await session.downloads.restore(viewer: viewer, server: client.server, client: client) }
            } }
            Button("Cancel", role: .cancel) {}
        } message: { Text("This removes downloaded media for every Viewer Profile on this device. Your Server library stays available.") }
        .alert("Remove download?", isPresented: Binding(get: { removal != nil }, set: { if !$0 { removal = nil } }), presenting: removal) { download in
            Button("Remove download", role: .destructive) { perform { try await session.downloads.remove(id: download.id) } }
            Button("Cancel", role: .cancel) {}
        } message: { download in Text("Remove \(download.item.title) from this device? The title stays in your Server library.") }
        #else
        ContentUnavailableView("Download on iPhone or iPad", systemImage: "iphone", description: Text("Use Kinosail on your iPhone or iPad to save media for offline playback."))
        #endif
    }
    #if os(iOS)
    private func bytes(_ value: Int64) -> String { ByteCountFormatter.string(fromByteCount: value, countStyle: .file) }
    private func transferDetail(_ download: OfflineDownload) -> String {
        var parts = ["\(bytes(download.receivedBytes)) of \(bytes(download.totalBytes))"]
        if let rate = download.bytesPerSecond {
            parts.append("\(bytes(Int64(rate)))/s")
            parts.append("\(remaining(Double(download.totalBytes - download.receivedBytes) / rate)) left")
        }
        return parts.joined(separator: " · ")
    }
    private func remaining(_ seconds: Double) -> String {
        let minutes = Int((seconds / 60).rounded(.up))
        return minutes < 1 ? "under a minute" : minutes < 60 ? "\(minutes) min" : "\(minutes / 60) h \(minutes % 60) min"
    }
    private func preparingDetail(_ download: OfflineDownload, now: Date) -> String {
        let what = download.quality == .original ? "Packaging the original file" : "Making a \(download.quality.title.lowercased()) copy"
        let percent = download.preparingPercent.map { "\($0)% · " } ?? ""
        guard let since = download.preparingSince else { return "\(percent)\(what). This can take several minutes." }
        let seconds = max(0, now.timeIntervalSince(since))
        if let done = download.preparingPercent, done >= 3 { return "\(percent)\(what) · about \(remaining(seconds * Double(100 - done) / Double(done))) left" }
        return "\(percent)\(what) · \(Int(seconds / 60)) min \(Int(seconds) % 60)s so far. This can take several minutes."
    }
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

    init(item: MediaItem) {
        self.item = item
        _quality = State(initialValue: item.isAudio ? .audio : .compatible)
    }

    var body: some View {
        #if os(iOS)
        Form {
            Section(item.title) {
                Picker("Quality", selection: $quality) {
                    ForEach(qualities) { value in Text(value.title).tag(value) }
                }
                Text(qualityNote).font(.caption).foregroundStyle(.secondary)
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
            Section {
                Button(busy ? "Adding download…" : "Add to Downloads") { enqueue() }.disabled(busy || !loaded || quality != .original && item.kind == .video && options == nil)
                    .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).controlSize(.large)
                    .tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                Text("Downloads use your saved network and storage preferences.").font(.caption).foregroundStyle(.secondary)
            }
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
    private var qualityNote: String {
        switch quality {
        case .compatible: "Keeps the video as-is when this device can already play it, so the file can be as big as the original. Choose 1080p or 720p for a smaller file."
        case .fullHD, .hd: "Re-encoded on your Server to H.264 at up to \(quality.title). Smaller, but preparation takes longer."
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
