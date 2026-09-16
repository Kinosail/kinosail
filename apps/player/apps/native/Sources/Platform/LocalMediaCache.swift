import Foundation
import CryptoKit

/// Disposable, profile-scoped data. Only validated catalog JSON and artwork enter
/// this store; credentials, playback URLs with capabilities, and media files do not.
actor LocalMediaCache {
    enum Kind: String, Sendable {
        case catalog, artwork
        var maximum: Int { self == .catalog ? 2 * 1024 * 1024 : 32 * 1024 * 1024 }
        var budget: Int { self == .catalog ? 64 * 1024 * 1024 : 256 * 1024 * 1024 }
        var count: Int { self == .catalog ? 512 : 2048 }
    }
    struct Entry: Sendable {
        let data: Data
        let saved: Date
        let fresh: Bool
    }
    private let root: URL
    private let scope: String
    private var closed = false
    private(set) var revision = UUID()
    private var freshAfter: Date?
    private var memory: [String: Entry] = [:]
    private var memoryBytes = 0
    private let manager = FileManager.default
    private let magic = Data("KINOCA01".utf8)

    init(scope: String, directory: URL? = nil) throws {
        self.scope = try Input.hex(scope, count: 64)
        let base = directory ?? FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("KinosailMediaCache", isDirectory: true)
        root = base.resolvingSymlinksInPath().appendingPathComponent(scope, isDirectory: true)
    }

    func read(_ key: String, kind: Kind) -> Entry? {
        guard !closed else { return nil }
        do {
            let file = try location(key, kind: kind)
            let id = file.lastPathComponent
            if kind == .catalog, let entry = memory[id] { return entryWithFreshness(entry, kind: kind) }
            let data = try boundedData(file, maximum: kind.maximum + 48)
            guard data.count >= 48, data.prefix(40) == header(key, kind: kind) else { return nil }
            let seconds = Double(bitPattern: UInt64(bigEndian: data.withUnsafeBytes { $0.loadUnaligned(fromByteOffset: 40, as: UInt64.self) }))
            let age = Date().timeIntervalSince1970 - seconds
            guard seconds.isFinite, age >= -60, age <= 30 * 86_400 else { return nil }
            let entry = Entry(data: Data(data.dropFirst(48)), saved: Date(timeIntervalSince1970: seconds), fresh: false)
            if kind == .catalog { remember(entry, id: id) }
            return entryWithFreshness(entry, kind: kind)
        } catch { return nil }
    }

    func write(_ data: Data, key: String, kind: Kind, revision expectedRevision: UUID? = nil) throws {
        guard !closed, expectedRevision == nil || expectedRevision == revision else { return }
        let file = try location(key, kind: kind)
        guard !data.isEmpty, data.count <= kind.maximum else { throw ClientError.invalidResponse }
        try prepareDirectory()
        let saved = Date()
        var output = header(key, kind: kind)
        var timestamp = saved.timeIntervalSince1970.bitPattern.bigEndian
        withUnsafeBytes(of: &timestamp) { output.append(contentsOf: $0) }
        output.append(data)
        try trim(kind, replacing: file, bytes: output.count)
        try output.write(to: file, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
        if kind == .catalog { remember(Entry(data: data, saved: saved, fresh: true), id: file.lastPathComponent) }
    }

    func remove(_ key: String, kind: Kind) {
        guard let file = try? location(key, kind: kind) else { return }
        forget(file.lastPathComponent)
        try? manager.removeItem(at: file)
    }

    /// Keep the saved screen visible, but require a refresh after a local mutation.
    func invalidateCatalog() {
        revision = UUID()
        freshAfter = Date()
        guard !closed else { return }
        do {
            try prepareDirectory()
            let file = root.appendingPathComponent("catalog-invalidation")
            guard file.resolvingSymlinksInPath().path == file.path else { return }
            var bits = freshAfter!.timeIntervalSince1970.bitPattern.bigEndian
            let data = withUnsafeBytes(of: &bits) { Data($0) }
            try data.write(to: file, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
        } catch { /* Cache failure must not turn a successful Server mutation into failure. */ }
    }

    func close(purge: Bool) {
        closed = true
        memory = [:]; memoryBytes = 0
        if purge, root.resolvingSymlinksInPath().path == root.path { try? manager.removeItem(at: root) }
    }

    private func entryWithFreshness(_ entry: Entry, kind: Kind) -> Entry? {
        let age = Date().timeIntervalSince(entry.saved)
        guard age >= -60, age <= 30 * 86_400 else { return nil }
        if freshAfter == nil {
            let file = root.appendingPathComponent("catalog-invalidation")
            if let data = try? boundedData(file, maximum: 8), data.count == 8 {
                let seconds = Double(bitPattern: UInt64(bigEndian: data.withUnsafeBytes { $0.loadUnaligned(as: UInt64.self) }))
                freshAfter = seconds.isFinite && seconds >= 0 && seconds <= Date().timeIntervalSince1970 + 60 ? Date(timeIntervalSince1970: seconds) : Date()
            } else { freshAfter = manager.fileExists(atPath: file.path) ? Date() : .distantPast }
        }
        return Entry(data: entry.data, saved: entry.saved, fresh: kind == .artwork || age < 300 && entry.saved >= freshAfter!)
    }

    private func header(_ key: String, kind: Kind) -> Data {
        magic + Data(SHA256.hash(data: Data("\(scope)\n\(kind.rawValue)\n\(key)".utf8)))
    }

    private func location(_ key: String, kind: Kind) throws -> URL {
        guard !key.isEmpty, key.utf8.count <= 16_384, key.rangeOfCharacter(from: .controlCharacters) == nil,
              root.resolvingSymlinksInPath().path == root.path else { throw ClientError.invalidResponse }
        let hash = SHA256.hash(data: Data(key.utf8)).map { String(format: "%02x", $0) }.joined()
        let file = root.appendingPathComponent("\(kind.rawValue)-\(hash).cache")
        guard file.resolvingSymlinksInPath().path == file.path else { throw ClientError.invalidResponse }
        return file
    }

    private func boundedData(_ file: URL, maximum: Int) throws -> Data {
        let values = try file.resourceValues(forKeys: [.isRegularFileKey, .fileSizeKey])
        guard file.resolvingSymlinksInPath().path == file.path, values.isRegularFile == true,
              let size = values.fileSize, size <= maximum else { throw ClientError.invalidResponse }
        let handle = try FileHandle(forReadingFrom: file)
        defer { try? handle.close() }
        let data = try handle.read(upToCount: maximum + 1) ?? Data()
        guard data.count <= maximum else { throw ClientError.invalidResponse }
        return data
    }

    private func prepareDirectory() throws {
        guard root.resolvingSymlinksInPath().path == root.path else { throw ClientError.invalidResponse }
        try manager.createDirectory(at: root, withIntermediateDirectories: true,
                                    attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication])
        var directory = root
        var values = URLResourceValues(); values.isExcludedFromBackup = true
        try directory.setResourceValues(values)
    }

    private func trim(_ kind: Kind, replacing: URL, bytes: Int) throws {
        let keys: [URLResourceKey] = [.isRegularFileKey, .fileSizeKey, .contentModificationDateKey]
        guard let listing = manager.enumerator(at: root, includingPropertiesForKeys: keys, options: [.skipsSubdirectoryDescendants]) else { throw ClientError.invalidResponse }
        var entries: [(URL, Int, Date)] = []
        var seen = 0
        for case let file as URL in listing {
            seen += 1
            guard seen <= 4096 else { throw ClientError.invalidResponse }
            guard file != replacing, file.lastPathComponent.hasPrefix(kind.rawValue + "-"), file.pathExtension == "cache",
                  file.resolvingSymlinksInPath().path == file.path else { continue }
            let values = try file.resourceValues(forKeys: Set(keys))
            guard values.isRegularFile == true, let size = values.fileSize else { continue }
            entries.append((file, size, values.contentModificationDate ?? .distantPast))
        }
        var total = entries.reduce(bytes) { $0 + $1.1 }
        var count = entries.count + 1
        for entry in entries.sorted(by: { $0.2 < $1.2 }) where total > kind.budget || count > kind.count {
            try manager.removeItem(at: entry.0)
            forget(entry.0.lastPathComponent)
            total -= entry.1; count -= 1
        }
    }

    private func remember(_ entry: Entry, id: String) {
        forget(id)
        while memoryBytes + entry.data.count > 8 * 1024 * 1024 || memory.count >= 64 {
            guard let oldest = memory.min(by: { $0.value.saved < $1.value.saved }) else { break }
            forget(oldest.key)
        }
        memory[id] = entry; memoryBytes += entry.data.count
    }
    private func forget(_ id: String) {
        if let previous = memory.removeValue(forKey: id) { memoryBytes -= previous.data.count }
    }
}
