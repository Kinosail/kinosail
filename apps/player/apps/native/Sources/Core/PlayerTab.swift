import Foundation

enum PlayerTab: String, CaseIterable, Identifiable, Sendable {
    case movies, shows, home, search, list, library, music, audiobooks, books, photos, collections, downloads, settings, more
    var id: String { rawValue }
    static let defaults: [Self] = [.home, .shows, .movies, .search]
    static func legacyDefault(_ raw: String?) -> String {
        guard let raw, let items = try? parse(raw), items != [.movies, .shows] else {
            return defaults.map(\.rawValue).joined(separator: ",")
        }
        return items.map(\.rawValue).joined(separator: ",")
    }
    static var available: [Self] {
        #if os(tvOS)
        allCases.filter { $0 != .more && $0 != .downloads && $0 != .books }
        #else
        allCases.filter { $0 != .more }
        #endif
    }
    static func parse(_ raw: String) throws -> [Self] {
        guard raw.utf8.count <= 128 else { throw ClientError.invalidInput("Choose up to four tabs.") }
        let parts = raw.split(separator: ",", omittingEmptySubsequences: false)
        let items = parts.compactMap { Self(rawValue: String($0)) }
        guard (1...4).contains(parts.count), items.count == parts.count,
              Set(items).count == items.count, items.allSatisfy(available.contains) else {
            throw ClientError.invalidInput("Choose one to four different tabs.")
        }
        return items
    }
    var title: String {
        switch self {
        case .movies: "Movies"
        case .shows: "TV Shows"
        case .home: "Home"
        case .search: "Search"
        case .list: "My List"
        case .library: "Library"
        case .music: "Music"
        case .audiobooks: "Audiobooks"
        case .books: "Books"
        case .photos: "Photos"
        case .collections: "Collections"
        case .downloads: "Downloads"
        case .settings: "Settings"
        case .more: "More"
        }
    }
    var symbol: String {
        switch self {
        case .movies: "film"
        case .shows: "tv"
        case .home: "house"
        case .search: "magnifyingglass"
        case .list: "bookmark"
        case .library: "square.grid.2x2"
        case .music: "music.note"
        case .audiobooks: "headphones"
        case .books: "book"
        case .photos: "photo"
        case .collections: "square.stack"
        case .downloads: "arrow.down.circle"
        case .settings: "gearshape"
        case .more: "ellipsis"
        }
    }
}
