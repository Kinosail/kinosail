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
            let all = session.downloads.downloads
            if !all.isEmpty && !session.downloads.restoring {
                Section { DownloadStorageSummary(downloads: all) }
            }
            let ready = all.filter { $0.state == .ready }, pending = all.filter { $0.state != .ready }
            if !pending.isEmpty {
                Section("In progress") { ForEach(pending) { row($0) } }
            }
            if !ready.isEmpty {
                Section("Ready to watch") { ForEach(ready) { row($0) } }
            }
        }
        .navigationTitle("Downloads")
        .listCanvas()
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
    private func row(_ download: OfflineDownload) -> some View {
        DownloadRow(download: download, compact: dynamicType.isAccessibilitySize, offline: session.connection.unavailable,
                    disabled: busy || session.downloads.busy, toggle: { toggle(download) }, remove: { removal = download })
    }
    private func toggle(_ download: OfflineDownload) {
        if [.paused, .failed].contains(download.state) { perform { try await session.downloads.resume(id: download.id) } }
        else if download.inProgress && download.state != .verifying { perform { try await session.downloads.pause(id: download.id) } }
    }
    private func perform(_ action: @escaping @MainActor () async throws -> Void) {
        guard !busy else { return }
        busy = true
        Task { defer { busy = false }; do { try await action(); failure = nil } catch { failure = AppSession.message(error) } }
    }
    #endif
}
