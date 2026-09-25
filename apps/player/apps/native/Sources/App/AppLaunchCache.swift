import Foundation

struct LibrarySnapshot {
    let items: [MediaItem]
    let page: LibraryPage
    let revision: UUID?

    static func key(view: LibraryView, sort: LibrarySort = .title, query: String = "") -> String {
        "library:\(view.rawValue):\(sort.rawValue):\(query)"
    }
}

@MainActor
enum AppLaunchCache {
    /// Decode only the first visible tab before the shell appears. The screen
    /// then revalidates this saved content without holding up its first frame.
    static func hydrate(client: ServerClient, profileKey: String, snapshots: ResourceSnapshotCache,
                        defaults: UserDefaults = .standard, ifCurrent: () -> Bool = { true }) async {
        guard ifCurrent() else { return }
        let mode = PlayerMode.stored(defaults.string(forKey: PlayerMode.storageKey(profileKey)))
        let tabs = defaults.string(forKey: mode.tabsKey(profileKey))
        let first = tabs.flatMap { try? PlayerTab.parse($0).first } ?? mode.defaultTabs[0]
        let clientID = client.identity
        switch first {
        case .home:
            if let home = try? await client.home(mode: mode, policy: .cached), ifCurrent() {
                snapshots.store(home, for: "\(profileKey):\(mode.rawValue)", clientID: clientID)
            }
        case .music:
            if let albums = try? await client.albums(policy: .cached), ifCurrent() {
                snapshots.store(albums, for: profileKey, clientID: clientID)
            }
        case .collections:
            if let collections = try? await client.collections(policy: .cached), ifCurrent() {
                snapshots.store(collections, for: profileKey, clientID: clientID)
            }
        case .movies, .shows, .search, .list, .audiobooks, .books, .photos:
            guard let view = first == .search ? mode.searchViews.first : LibraryView(rawValue: first.rawValue),
                  let page = try? await client.library(view: view, policy: .cached), ifCurrent() else { return }
            snapshots.store(LibrarySnapshot(items: page.items, page: page, revision: nil),
                            for: LibrarySnapshot.key(view: view), clientID: clientID)
        case .library, .downloads, .settings, .more: break
        }
    }
}
