import SwiftUI

struct MusicScreen: View {
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @Environment(AppSession.self) private var session
    #if os(tvOS)
    @Namespace private var albumsFocus
    @FocusState private var focusedControl: MusicFocus?
    @State private var requestedInitialFocus = false
    private enum MusicFocus: Hashable { case tracks, album(String) }
    #endif
    var body: some View {
        ScrollView {
            ResourceView(identity: session.profileKey ?? "", loadingLayout: .musicGrid, load: { policy in
                guard let client = session.client else { throw ClientError.unavailable }
                return try await client.albums(policy: policy)
            }) { albums in
                #if os(tvOS)
                HStack {
                    Text("Albums").font(.system(.largeTitle, design: .rounded).bold())
                        .accessibilityAddTraits(.isHeader)
                    Spacer()
                    NavigationLink("All music tracks", value: ScreenDestination.library(.music))
                        .buttonStyle(.bordered).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
                        .focused($focusedControl, equals: .tracks)
                }
                .focusSection()
                #else
                NavigationLink("All music tracks", value: ScreenDestination.library(.music))
                    .frame(minHeight: 44)
                #endif
                if albums.isEmpty { ContentUnavailableView("No albums yet", systemImage: "music.note", description: Text("Tracks without album information are available in All music tracks.")) }
                LazyVGrid(columns: MediaGrid.columns(landscape: false, accessibility: dynamicTypeSize.isAccessibilitySize), spacing: 28) {
                    ForEach(albums) { album in
                        NavigationLink(value: ScreenDestination.album(album.id)) {
                            VStack(alignment: .leading, spacing: 10) {
                                Artwork(path: album.artwork, symbol: "music.note", ratio: 1).clipShape(.rect(cornerRadius: 12))
                                VStack(alignment: .leading, spacing: 10) {
                                    Text(album.title).font(KinoTheme.cardTitle).lineLimit(dynamicTypeSize.isAccessibilitySize ? nil : 2)
                                    Text(album.artist).font(.caption).foregroundStyle(KinoTheme.muted).lineLimit(dynamicTypeSize.isAccessibilitySize ? nil : 1)
                                }
                                #if os(tvOS)
                                .padding([.horizontal, .bottom], 12)
                                #endif
                            }
                        }
                        #if os(iOS)
                        .buttonStyle(.plain)
                        #else
                        .buttonStyle(.card)
                        .tvOSDefaultPlayFocus(in: albumsFocus, id: "albums.first.\(album.id)", enabled: albums.first?.id == album.id)
                        .focused($focusedControl, equals: .album(album.id))
                        #endif
                    }
                }
                #if os(tvOS)
                .padding(.vertical, 24)
                .focusSection()
                .task(id: albums.first?.id) {
                    guard !requestedInitialFocus else { return }
                    requestedInitialFocus = true
                    if focusedControl == nil { focusedControl = albums.first.map { .album($0.id) } ?? .tracks }
                }
                #endif
            }.padding(KinoTheme.contentPadding)
        }
        .cinemaBackground().tabBarCollapses()
        #if os(tvOS)
        .focusScope(albumsFocus)
        .navigationTitle("")
        #else
        .navigationTitle("Albums")
        #endif
    }
}

struct AlbumScreen: View {
    let albumID: String
    @Environment(AppSession.self) private var session
    @State private var message: String?
    @State private var starting = false
    @State private var showsPlayer = false
    #if os(tvOS)
    @State private var presentedWhileStarting = false
    @Namespace private var albumFocus
    #endif

    var body: some View {
        ScrollView {
            ResourceView(identity: "\(session.profileKey ?? ""):\(albumID)", loadingLayout: .album, load: { policy in
                guard let client = session.client else { throw ClientError.unavailable }
                return try await client.album(id: albumID, policy: policy)
            }) { album in
                VStack(alignment: .leading, spacing: 24) {
                    #if os(tvOS)
                    HStack(alignment: .bottom, spacing: 32) {
                        if let first = album.tracks.first {
                            Artwork(path: first.artwork, symbol: "music.note", ratio: 1)
                                .frame(width: 260).clipShape(.rect(cornerRadius: 16))
                        }
                        VStack(alignment: .leading, spacing: 8) {
                            Text(album.title).font(.system(.largeTitle, design: .rounded).bold())
                                .accessibilityAddTraits(.isHeader)
                            Text(album.artist).font(.title3).foregroundStyle(KinoTheme.muted)
                            Text("\(album.tracks.count) \(album.tracks.count == 1 ? "track" : "tracks")")
                                .font(.callout).foregroundStyle(KinoTheme.muted)
                        }
                    }
                    #else
                    if let first = album.tracks.first {
                        Artwork(path: first.artwork, symbol: "music.note", ratio: 1)
                            .frame(maxWidth: 280).clipShape(.rect(cornerRadius: 16))
                            .shadow(color: .black.opacity(0.35), radius: 18, y: 8)
                    }
                    VStack(alignment: .leading, spacing: 6) {
                        Text(album.title).font(.system(.largeTitle, design: .rounded).bold())
                            .fixedSize(horizontal: false, vertical: true)
                            .accessibilityAddTraits(.isHeader)
                        Text([album.artist, "\(album.tracks.count) \(album.tracks.count == 1 ? "track" : "tracks")"]
                            .filter { !$0.isEmpty }.joined(separator: " · "))
                            .font(.subheadline.weight(.medium)).foregroundStyle(KinoTheme.muted)
                    }
                    if !album.tracks.isEmpty {
                        Button { play(album.tracks, at: 0) } label: {
                            Label("Play", systemImage: "play.fill").padding(.horizontal, 6)
                        }
                        .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).controlSize(.large)
                        .tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                        .disabled(starting)
                    }
                    #endif
                    if let message { Text(message).foregroundStyle(KinoTheme.muted) }
                    if album.tracks.isEmpty { ContentUnavailableView("No tracks", systemImage: "music.note") }
                    #if os(tvOS)
                    if !album.tracks.isEmpty { Text("Tracks").font(.title2.bold()).accessibilityAddTraits(.isHeader) }
                    #endif
                    ForEach(Array(album.tracks.enumerated()), id: \.element.id) { index, item in
                        Button { play(album.tracks, at: index) } label: {
                            HStack(spacing: 20) {
                                Text("\(index + 1)").monospacedDigit().foregroundStyle(KinoTheme.muted).frame(minWidth: 30)
                                VStack(alignment: .leading) { Text(item.title).font(.headline); Text(item.artist).font(.caption).foregroundStyle(KinoTheme.muted) }
                                Spacer()
                                Image(systemName: session.player.currentItem?.id == item.id && session.player.isPlaying ? "waveform" : "play.fill")
                            }.padding(.vertical, 12).contentShape(.rect)
                        }
                        #if os(iOS)
                        .buttonStyle(.plain)
                        #else
                        .buttonStyle(.card)
                        #endif
                        .disabled(starting)
                        #if os(tvOS)
                        .tvOSDefaultPlayFocus(in: albumFocus, id: "album.first-track.\(albumID).\(item.id)", enabled: index == 0)
                        #endif
                        Divider()
                    }
                }.frame(maxWidth: 1100, alignment: .leading).frame(maxWidth: .infinity)
            }.padding(KinoTheme.contentPadding)
        }
        .cinemaBackground().tabBarCollapses()
        #if os(tvOS)
        .navigationTitle("")
        #else
        .navigationTitle("")
        .navigationBarTitleDisplayMode(.inline)
        #endif
        #if os(tvOS)
        .focusScope(albumFocus)
        .onChange(of: session.player.loading) { _, loading in
            if starting && loading { presentedWhileStarting = true; showsPlayer = true }
        }
        #endif
        #if os(tvOS)
        .fullScreenCover(isPresented: $showsPlayer) {
            if let item = session.player.currentItem { NavigationStack { AudioPlayerScreen(itemID: item.id) } }
        }
        #else
        .sheet(isPresented: $showsPlayer) { if let item = session.player.currentItem { NavigationStack { AudioPlayerScreen(itemID: item.id, hidesMiniPlayer: false) }.presentationSizing(.page) } }
        #endif
    }

    private func play(_ items: [MediaItem], at index: Int) {
        guard let client = session.client, let store = session.progress, !starting else { return }
        starting = true
        #if os(tvOS)
        presentedWhileStarting = false
        #endif
        Task {
            defer { starting = false }
            do {
                try await session.player.playQueue(items, at: index, client: client, store: store)
                #if os(tvOS)
                if !presentedWhileStarting { showsPlayer = true }
                #else
                showsPlayer = true
                #endif
                message = nil
            }
            catch { message = AppSession.message(error) }
        }
    }
}
