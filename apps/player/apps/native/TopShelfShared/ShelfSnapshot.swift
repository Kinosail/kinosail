import Foundation

struct ShelfSnapshot: Codable, Sendable {
    enum Section: String, CaseIterable, Sendable {
        case continueWatching = "Continue watching"
        case myList = "My List"
        case recentlyAdded = "Recently added"
    }

    struct Item: Codable, Sendable {
        let id: String
        let title: String
        let section: String
        let image: Data
    }
    let scope: String
    let saved: Date
    let items: [Item]
    static let group = "group.com.kinosail.player"
    static let maximum = 3 * 1024 * 1024
    static let maximumImage = 90 * 1024

    static func directory() -> URL? {
        FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: group)?
            .appendingPathComponent("Library/Caches/KinosailTopShelf", isDirectory: true)
    }

    static func clear() {
        guard let root = directory() else { return }
        try? FileManager.default.removeItem(at: root)
    }

    func validated() throws -> Self {
        guard scope.count == 64, scope.utf8.allSatisfy({ (48...57).contains($0) || (97...102).contains($0) }),
              saved.timeIntervalSinceNow <= 60, saved.timeIntervalSinceNow >= -86_400,
              items.count <= 18, Set(items.map { $0.section + ":" + $0.id }).count == items.count else { throw CocoaError(.fileReadCorruptFile) }
        for item in items {
            guard !item.id.isEmpty, item.id.utf8.count <= 128,
                  item.id.utf8.allSatisfy({ (48...57).contains($0) || (65...90).contains($0) || (97...122).contains($0) || $0 == 45 || $0 == 95 }),
                  !item.title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty, item.title.utf8.count <= 2048,
                  item.title.rangeOfCharacter(from: .controlCharacters) == nil,
                  Section.allCases.map(\.rawValue).contains(item.section),
                  item.image.count <= Self.maximumImage, item.image.starts(with: [0xff, 0xd8, 0xff]) else { throw CocoaError(.fileReadCorruptFile) }
        }
        return self
    }

    static func read() throws -> Self? {
        guard let root = directory() else { return nil }
        let file = root.appendingPathComponent("snapshot.json")
        guard FileManager.default.fileExists(atPath: file.path) else { return nil }
        guard root.resolvingSymlinksInPath() == root.standardizedFileURL,
              file.resolvingSymlinksInPath() == file.standardizedFileURL,
              let size = try file.resourceValues(forKeys: [.fileSizeKey]).fileSize, size <= maximum else { throw CocoaError(.fileReadCorruptFile) }
        let data = try Data(contentsOf: file)
        guard data.count <= maximum else { throw CocoaError(.fileReadCorruptFile) }
        return try decode(data)
    }

    static func decode(_ data: Data) throws -> Self {
        let root = try StrictJSON.decode(data, maximum: maximum).object(allowing: ["scope", "saved", "items"])
        for item in try root.required("items").array(max: 18) {
            _ = try item.object(allowing: ["id", "title", "section", "image"])
        }
        return try JSONDecoder().decode(Self.self, from: data).validated()
    }

    func write(to directory: URL? = Self.directory()) throws {
        _ = try validated()
        let data = try JSONEncoder().encode(self)
        guard data.count <= Self.maximum, let root = directory else { throw CocoaError(.fileWriteUnknown) }
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        guard root.resolvingSymlinksInPath() == root.standardizedFileURL else { throw CocoaError(.fileWriteUnknown) }
        try data.write(to: root.appendingPathComponent("snapshot.json"), options: [.atomic, .completeFileProtectionUnlessOpen])
    }

    func url(for item: Item, play: Bool) -> URL? {
        var parts = URLComponents()
        parts.scheme = "kinosail"; parts.host = "media"
        parts.queryItems = [URLQueryItem(name: "action", value: play ? "play" : "detail"),
                            URLQueryItem(name: "value", value: item.id), URLQueryItem(name: "scope", value: scope)]
        return parts.url
    }
}
