import SwiftUI

struct MusicScreen: View {
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @Environment(AppSession.self) private var session
    #if os(tvOS)
    @Namespace private var albumsFocus
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
                        .buttonStyle(.bordered).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.secondaryControlInk)
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
                                Text(album.title).font(.headline).lineLimit(dynamicTypeSize.isAccessibilitySize ? nil : 2)
                                Text(album.artist).font(.caption).foregroundStyle(.secondary).lineLimit(dynamicTypeSize.isAccessibilitySize ? nil : 1)
                            }
                        }
                        #if os(iOS)
                        .buttonStyle(.plain)
                        #else
                        .buttonStyle(.card)
                        .tvOSDefaultPlayFocus(in: albumsFocus, id: "albums.first.\(album.id)", enabled: albums.first?.id == album.id)
                        #endif
                    }
                }
                #if os(tvOS)
                .padding(.vertical, 24)
                .focusSection()
                #endif
            }.padding(KinoTheme.contentPadding)
        }
        .cinemaBackground()
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
                    if let first = album.tracks.first { Artwork(path: first.artwork, symbol: "music.note", ratio: 1).frame(maxWidth: 360).clipShape(.rect(cornerRadius: 16)) }
                    Text(album.title).font(.largeTitle.bold())
                    Text(album.artist).font(.headline).foregroundStyle(.secondary)
                    #endif
                    if let message { Text(message).foregroundStyle(.secondary) }
                    if album.tracks.isEmpty { ContentUnavailableView("No tracks", systemImage: "music.note") }
                    #if os(tvOS)
                    if !album.tracks.isEmpty { Text("Tracks").font(.title2.bold()).accessibilityAddTraits(.isHeader) }
                    #endif
                    ForEach(Array(album.tracks.enumerated()), id: \.element.id) { index, item in
                        Button { play(album.tracks, at: index) } label: {
                            HStack(spacing: 20) {
                                Text("\(index + 1)").monospacedDigit().foregroundStyle(.secondary).frame(minWidth: 30)
                                VStack(alignment: .leading) { Text(item.title).font(.headline); Text(item.artist).font(.caption).foregroundStyle(.secondary) }
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
        .cinemaBackground()
        #if os(tvOS)
        .navigationTitle("")
        #else
        .navigationTitle("Album")
        #endif
        #if os(tvOS)
        .focusScope(albumFocus)
        #endif
        .sheet(isPresented: $showsPlayer) { if let item = session.player.currentItem { NavigationStack { AudioPlayerScreen(itemID: item.id) }.presentationSizing(.page) } }
    }

    private func play(_ items: [MediaItem], at index: Int) {
        guard let client = session.client, let store = session.progress, !starting else { return }
        starting = true
        Task {
            defer { starting = false }
            do { try await session.player.playQueue(items, at: index, client: client, store: store); showsPlayer = true; message = nil }
            catch { message = AppSession.message(error) }
        }
    }
}
