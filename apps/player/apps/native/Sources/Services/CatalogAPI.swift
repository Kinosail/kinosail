import Foundation

extension ServerClient {
    func warmCatalog(mode: PlayerMode? = nil, landingTab: PlayerTab = .home,
                     onHome: (@Sendable (PlayerMode, HomeSnapshot, Bool) async -> Void)? = nil) async {
        guard !Task.isCancelled else { return }
        if let mode {
            if let onHome {
                for cachedMode in [mode.other, mode] {
                    guard !Task.isCancelled else { return }
                    if let saved = try? await home(mode: cachedMode, policy: .cached) {
                        await onHome(cachedMode, saved, false)
                    }
                }
            }
            switch landingTab {
            case .movies: _ = try? await library(view: .movies, policy: .automatic)
            case .shows: _ = try? await library(view: .shows, policy: .automatic)
            case .search: _ = try? await library(view: mode.other.searchViews[0], policy: .automatic)
            case .list: _ = try? await library(view: .list, policy: .automatic)
            case .music: _ = try? await albums(policy: .automatic)
            case .audiobooks: _ = try? await library(view: .audiobooks, policy: .automatic)
            case .books: _ = try? await library(view: .books, policy: .automatic)
            case .photos: _ = try? await library(view: .photos, policy: .automatic)
            case .collections: _ = try? await collections(policy: .automatic)
            case .home, .library, .downloads, .settings, .more: break
            }
            guard !Task.isCancelled else { return }
            if let warmed = try? await home(mode: mode.other, policy: .automatic) {
                await onHome?(mode.other, warmed, true)
            }
            guard !Task.isCancelled else { return }
            _ = try? await home(mode: mode, policy: .automatic)
        } else { _ = try? await home(policy: .automatic) }
        for view in [LibraryView.movies, .shows] {
            guard !Task.isCancelled else { return }
            _ = try? await library(view: view, policy: .automatic)
        }
    }

    func home(mode: PlayerMode? = nil, policy: CatalogPolicy = .reload) async throws -> HomeSnapshot {
        let profile: Viewer
        if let associatedViewer { profile = associatedViewer }
        else if policy == .cached { throw CatalogCacheMiss.missing }
        else { profile = try await viewer() }
        async let history = library(view: .history, limit: mode == nil ? 24 : 200, policy: policy)
        if let mode {
            async let first = library(view: mode.searchViews[0], sort: .added, limit: 36, policy: policy)
            async let second = library(view: mode.searchViews[1], sort: .added, limit: 36, policy: policy)
            let (historyPage, firstPage, secondPage) = try await (history, first, second)
            let continuing = historyPage.items.filter { $0.progress.seconds > 0 && !$0.progress.watched && !$0.progress.dismissed && mode.includes($0) }
            return HomeSnapshot(viewer: profile, continueWatching: continuing, recent: firstPage.items + secondPage.items)
        }
        async let recent = library(sort: .added, limit: 36, policy: policy)
        let (historyPage, recentPage) = try await (history, recent)
        let continueWatching = historyPage.items.filter { $0.progress.seconds > 0 && !$0.progress.watched && !$0.progress.dismissed }
        return HomeSnapshot(viewer: profile, continueWatching: continueWatching, recent: recentPage.items)
    }

    func library(query: String = "", view: LibraryView = .all, sort: LibrarySort = .title,
                 offset: Int = 0, limit: Int = 60, policy: CatalogPolicy = .reload) async throws -> LibraryPage {
        let query = try Input.text(query, max: 512, label: "search", empty: true)
        guard (0...1_000_000).contains(offset), (1...200).contains(limit) else {
            throw ClientError.invalidInput("The requested library page is invalid.")
        }
        let path = "/api/v1/library?q=\(Input.segment(query))&view=\(view.rawValue)&sort=\(sort.rawValue)&offset=\(offset)&limit=\(limit)"
        return try await catalog(path, policy: policy) { [self] raw in
            let page = try LibraryPage(raw, server: server)
            guard page.offset == offset, page.limit == limit else { throw ClientError.invalidResponse }
            let fields = try raw.object()
            for (name, expected) in [("view", view.rawValue), ("sort", sort.rawValue), ("query", query)] {
                if fields[name] != nil, try fields.text(name, max: 512) != expected { throw ClientError.invalidResponse }
            }
            return page
        }
    }

    func details(id: String, policy: CatalogPolicy = .reload) async throws -> ItemDetail {
        let id = try Input.id(id)
        return try await catalog("/api/v1/items/\(id)", policy: policy) { [self] raw in
            let value = try raw.object(allowing: ["item", "listed", "profileId"])
            let item = try MediaItem(value.required("item"), server: server)
            guard item.id == id else { throw ClientError.invalidResponse }
            let profileID = try Input.id(value.text("profileId", max: 128, required: true))
            guard viewerID == nil || profileID == viewerID else { throw ClientError.invalidResponse }
            return try ItemDetail(item: item, listed: value.flag("listed"))
        }
    }

    func item(id: String, policy: CatalogPolicy = .reload) async throws -> MediaItem {
        try await details(id: id, policy: policy).item
    }

    func setListed(itemID: String, listed: Bool) async throws -> Bool {
        let value = try await request("/api/v1/items/\(Input.id(itemID))/list", method: .put,
                                      body: .object(["listed": .bool(listed)])).body.object(allowing: ["listed"])
        let saved = try value.flag("listed")
        guard saved == listed else { throw ClientError.invalidResponse }
        await invalidateCatalog()
        return saved
    }

    func watchProgress(itemID: String) async throws -> WatchProgressSummary {
        let id = try Input.id(itemID)
        return try await WatchProgressSummary(request("/api/v1/items/\(id)/watch-progress").body)
    }

    func dismissContinueWatching(itemID: String) async throws {
        _ = try await request("/api/v1/items/\(Input.id(itemID))/continue-watching", method: .delete, expected: [204])
        await invalidateCatalog()
    }

    func episodes(showID: String, policy: CatalogPolicy = .reload) async throws -> [MediaItem] {
        try await show(id: showID, policy: policy).episodes
    }

    func show(id showID: String, policy: CatalogPolicy = .reload) async throws -> ShowDetail {
        let id = try Input.hex(showID, count: 16)
        return try await catalog("/api/v1/shows/\(id)", policy: policy) { [self] raw in
            let value = try raw.object(allowing: [
                "id", "title", "backdrop", "play", "episodes", "cast", "year", "plot", "genres", "studio"
            ])
            guard try value.text("id", required: true) == id else { throw ClientError.invalidResponse }
            _ = try value.text("title", max: 512, required: true)
            let items = try mediaItems(value.required("episodes"))
            guard items.allSatisfy({ $0.showID == id && $0.kind == .video }) else { throw ClientError.invalidResponse }
            return try ShowDetail(episodes: items.sorted { ($0.season, $0.episode, $0.title) < ($1.season, $1.episode, $1.title) },
                                  cast: CastMember.list(value, server: server))
        }
    }

    func collections(policy: CatalogPolicy = .reload) async throws -> [String] {
        return try await catalog("/api/v1/collections", policy: policy) { raw in
            let value = try raw.object(allowing: ["collections", "summaries"])
            let names = try value.required("collections").array(max: 10_000).map { raw in
                guard case .string(let name) = raw else { throw ClientError.invalidResponse }
                return try Input.collection(name)
            }
            guard Set(names).count == names.count else { throw ClientError.invalidResponse }
            return names
        }
    }

    func collection(name: String, policy: CatalogPolicy = .reload) async throws -> [MediaItem] {
        let name = try Input.collection(name)
        return try await catalog("/api/v1/collections/\(Input.segment(name))", policy: policy) { [self] raw in
            let value = try raw.object(allowing: ["name", "items"])
            guard try value.text("name", max: 64, required: true) == name else { throw ClientError.invalidResponse }
            return try mediaItems(value.required("items"))
        }
    }

    func albums(policy: CatalogPolicy = .reload) async throws -> [Album] {
        return try await catalog("/api/v1/albums", policy: policy) { [self] raw in
            let value = try raw.object(allowing: ["albums"])
            return try Input.unique(value.required("albums").array(max: 10_000).map { raw in
                let album = try raw.object(allowing: ["id", "title", "artist", "artwork"])
                let artwork = try album.text("artwork")
                if !artwork.isEmpty { _ = try server.mediaURL(artwork) }
                return try Album(id: Input.id(album.text("id", max: 128, required: true)),
                                 title: album.text("title", max: 512, required: true),
                                 artist: album.text("artist", max: 256), artwork: artwork)
            })
        }
    }

    func album(id: String, policy: CatalogPolicy = .reload) async throws -> AlbumDetail {
        let id = try Input.id(id)
        return try await catalog("/api/v1/albums/\(id)", policy: policy) { [self] raw in
            let value = try raw.object(allowing: ["id", "title", "artist", "tracks"])
            guard try value.text("id", required: true) == id else { throw ClientError.invalidResponse }
            let tracks = try mediaItems(value.required("tracks"))
            guard tracks.allSatisfy({ $0.kind == .music }) else { throw ClientError.invalidResponse }
            return try AlbumDetail(id: id, title: value.text("title", max: 512, required: true),
                                   artist: value.text("artist", max: 256), tracks: tracks)
        }
    }

    func musicQueue(item: MediaItem) async throws -> [MediaItem] {
        guard item.kind == .music else { throw ClientError.invalidInput("This title is not a music track.") }
        let id = try Input.id(item.id)
        // The Server only exposes queues for album tracks; loose tracks play on their own.
        if item.album.isEmpty { return [item] }
        let value = try await request("/api/v1/audio/\(id)/queue").body.object(allowing: ["items"])
        let items = try mediaItems(value.required("items"))
        guard items.first?.id == id, items.allSatisfy({ $0.kind == .music }) else { throw ClientError.invalidResponse }
        return items
    }

    private func mediaItems(_ raw: JSONValue) throws -> [MediaItem] {
        try Input.unique(raw.array(max: 10_000).map { try MediaItem($0, server: server) })
    }
}
