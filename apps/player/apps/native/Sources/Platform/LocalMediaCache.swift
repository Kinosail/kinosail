import Foundation
import CryptoKit

/// Disposable, profile-scoped data. Only validated catalog JSON and artwork enter
/// this store; credentials, playback URLs with capabilities, and media files do not.
actor LocalMediaCache {
    static let artworkFreshLifetime: TimeInterval = 24 * 60 * 60

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
    private struct DiskEntry {
        let file: URL
        let bytes: Int
        let modified: Date
    }
    private struct PendingWrite {
        let key: String
        let kind: Kind
        let task: Task<Void, Never>
    }
    private let root: URL
    private let scope: String
    private var closed = false
    private var closing = false
    private var pendingWrites: [UUID: PendingWrite] = [:]
    private var diskEntries: [Kind: [String: DiskEntry]] = [:]
    private(set) var revision = UUID()
    private(set) var pagesRevision = UUID()
    private var freshAfter: Date?
    private var pagesFreshAfter: Date?
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
            if kind == .catalog, let entry = memory[id] { return entryWithFreshness(entry, kind: kind, key: key) }
            let data = try boundedData(file, maximum: kind.maximum + 48)
            guard data.count >= 48, data.prefix(40) == header(key, kind: kind) else { return nil }
            let seconds = Double(bitPattern: UInt64(bigEndian: data.withUnsafeBytes { $0.loadUnaligned(fromByteOffset: 40, as: UInt64.self) }))
            let age = Date().timeIntervalSince1970 - seconds
            guard seconds.isFinite, age >= -60, age <= 30 * 86_400 else { return nil }
            let entry = Entry(data: Data(data.dropFirst(48)), saved: Date(timeIntervalSince1970: seconds), fresh: false)
            if kind == .catalog { remember(entry, id: id) }
            return entryWithFreshness(entry, kind: kind, key: key)
        } catch { return nil }
    }

    func write(_ data: Data, key: String, kind: Kind, revision expectedRevision: UUID? = nil,
               pagesRevision expectedPagesRevision: UUID? = nil) throws {
        guard !closed, expectedRevision == nil || expectedRevision == revision else { return }
        guard expectedPagesRevision == nil || expectedPagesRevision == pagesRevision else { return }
        let file = try location(key, kind: kind)
        guard !data.isEmpty, data.count <= kind.maximum else { throw ClientError.invalidResponse }
        try prepareDirectory()
        let saved = Date()
        var output = header(key, kind: kind)
        var timestamp = saved.timeIntervalSince1970.bitPattern.bigEndian
        withUnsafeBytes(of: &timestamp) { output.append(contentsOf: $0) }
        output.append(data)
        do {
            try trim(kind, replacing: file, bytes: output.count)
            try output.write(to: file, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
        } catch {
            diskEntries = [:]
            throw error
        }
        diskEntries[kind]?[file.lastPathComponent] = DiskEntry(file: file, bytes: output.count, modified: saved)
        if kind == .catalog { remember(Entry(data: data, saved: saved, fresh: true), id: file.lastPathComponent) }
    }

    func enqueueWrite(_ data: Data, key: String, kind: Kind, revision expectedRevision: UUID? = nil,
                      pagesRevision expectedPagesRevision: UUID? = nil) {
        guard !closed, !closing, pendingWrites.count < 128, !data.isEmpty, data.count <= kind.maximum,
              expectedRevision == nil || expectedRevision == revision,
              expectedPagesRevision == nil || expectedPagesRevision == pagesRevision,
              let file = try? location(key, kind: kind) else { return }
        if kind == .catalog { remember(Entry(data: data, saved: Date(), fresh: true), id: file.lastPathComponent) }
        let id = UUID()
        let task = Task(priority: .utility) {
            defer { pendingWrites[id] = nil }
            guard !Task.isCancelled else { return }
            try? write(data, key: key, kind: kind, revision: expectedRevision,
                       pagesRevision: expectedPagesRevision)
        }
        pendingWrites[id] = PendingWrite(key: key, kind: kind, task: task)
    }

    func flushWrites() async {
        for pending in Array(pendingWrites.values) { await pending.task.value }
    }

    func remove(_ key: String, kind: Kind) {
        for pending in pendingWrites.values where pending.key == key && pending.kind == kind { pending.task.cancel() }
        guard let file = try? location(key, kind: kind) else { return }
        forget(file.lastPathComponent)
        do {
            try manager.removeItem(at: file)
            diskEntries[kind]?[file.lastPathComponent] = nil
        } catch { diskEntries = [:] }
    }

    /// Keep the saved screen visible, but require a refresh after a local mutation.
    func invalidateCatalog() {
        revision = UUID()
        freshAfter = Date()
        saveInvalidation(freshAfter!, name: "catalog-invalidation")
    }

    /// A changed first page only makes later offsets stale, not sibling queries.
    func invalidatePages() {
        pagesRevision = UUID()
        pagesFreshAfter = Date()
        saveInvalidation(pagesFreshAfter!, name: "catalog-pages-invalidation")
    }

    private func saveInvalidation(_ date: Date, name: String) {
        guard !closed else { return }
        do {
            try prepareDirectory()
            let file = root.appendingPathComponent(name)
            guard file.resolvingSymlinksInPath().path == file.path else { return }
            var bits = date.timeIntervalSince1970.bitPattern.bigEndian
            let data = withUnsafeBytes(of: &bits) { Data($0) }
            try data.write(to: file, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
        } catch { /* Cache failure must not turn a successful Server mutation into failure. */ }
    }

    func close(purge: Bool) async {
        closing = true
        if purge { closed = true }
        await flushWrites()
        closed = true
        memory = [:]; memoryBytes = 0
        if purge, root.resolvingSymlinksInPath().path == root.path { try? manager.removeItem(at: root) }
    }

    private func entryWithFreshness(_ entry: Entry, kind: Kind, key: String) -> Entry? {
        let age = Date().timeIntervalSince(entry.saved)
        guard age >= -60, age <= 30 * 86_400 else { return nil }
        if kind == .artwork {
            return Entry(data: entry.data, saved: entry.saved, fresh: age < Self.artworkFreshLifetime)
        }
        if freshAfter == nil { freshAfter = invalidationDate("catalog-invalidation") }
        if pagesFreshAfter == nil { pagesFreshAfter = invalidationDate("catalog-pages-invalidation") }
        let laterPage = URLComponents(string: key)?.queryItems?.contains { $0.name == "offset" && $0.value != "0" } == true
        let cutoff = laterPage ? max(freshAfter!, pagesFreshAfter!) : freshAfter!
        return Entry(data: entry.data, saved: entry.saved, fresh: age < 60 && entry.saved >= cutoff)
    }

    private func invalidationDate(_ name: String) -> Date {
        let file = root.appendingPathComponent(name)
        guard let data = try? boundedData(file, maximum: 8), data.count == 8 else {
            return manager.fileExists(atPath: file.path) ? Date() : .distantPast
        }
        let seconds = Double(bitPattern: UInt64(bigEndian: data.withUnsafeBytes { $0.loadUnaligned(as: UInt64.self) }))
        return seconds.isFinite && seconds >= 0 && seconds <= Date().timeIntervalSince1970 + 60 ? Date(timeIntervalSince1970: seconds) : Date()
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
        if diskEntries.isEmpty { try indexDiskEntries() }
        var entries = diskEntries[kind] ?? [:]
        entries[replacing.lastPathComponent] = nil
        var total = entries.values.reduce(bytes) { $0 + $1.bytes }
        var count = entries.count + 1
        if total > kind.budget || count > kind.count {
            for entry in entries.values.sorted(by: { $0.modified < $1.modified }) where total > kind.budget || count > kind.count {
                try manager.removeItem(at: entry.file)
                forget(entry.file.lastPathComponent)
                entries[entry.file.lastPathComponent] = nil
                total -= entry.bytes; count -= 1
            }
        }
        diskEntries[kind] = entries
    }

    private func indexDiskEntries() throws {
        let keys: [URLResourceKey] = [.isRegularFileKey, .fileSizeKey, .contentModificationDateKey]
        guard let listing = manager.enumerator(at: root, includingPropertiesForKeys: keys, options: [.skipsSubdirectoryDescendants]) else { throw ClientError.invalidResponse }
        var entries: [Kind: [String: DiskEntry]] = [.catalog: [:], .artwork: [:]]
        var seen = 0
        for case let file as URL in listing {
            seen += 1
            guard seen <= 4096 else { throw ClientError.invalidResponse }
            guard file.pathExtension == "cache", file.resolvingSymlinksInPath().path == file.path,
                  let kind = [Kind.catalog, .artwork].first(where: { file.lastPathComponent.hasPrefix($0.rawValue + "-") }) else { continue }
            let values = try file.resourceValues(forKeys: Set(keys))
            guard values.isRegularFile == true, let size = values.fileSize else { continue }
            entries[kind]?[file.lastPathComponent] = DiskEntry(file: file, bytes: size, modified: values.contentModificationDate ?? .distantPast)
        }
        diskEntries = entries
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
