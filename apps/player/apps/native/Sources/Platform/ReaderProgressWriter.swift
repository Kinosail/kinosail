#if os(iOS)
import Foundation

struct PendingReading: Sendable {
    let itemID: String
    var position: ReaderPosition
    var expected: ReaderPosition
    var json: JSONValue { .object(["itemID": .string(itemID), "position": position.json, "expected": expected.json]) }
    init(itemID: String, position: ReaderPosition, expected: ReaderPosition) throws {
        self.itemID = try Input.id(itemID)
        self.position = try ReaderPosition(position.json)
        self.expected = try ReaderPosition(expected.json)
    }
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["itemID", "position", "expected"])
        try self.init(itemID: value.text("itemID", max: 128, required: true), position: ReaderPosition(value.required("position")), expected: ReaderPosition(value.required("expected")))
    }
}

extension ReaderPosition {
    var json: JSONValue { .object(["page": .number(Double(page)), "total": .number(Double(total)), "offset": .number(offset)]) }
}

actor ReaderPositionStore {
    private let scope: String
    private let file: URL
    private var entries: [PendingReading] = []
    private var loaded = false
    init(scope: String) throws {
        self.scope = try Input.hex(scope, count: 64)
        file = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].resolvingSymlinksInPath()
            .appendingPathComponent("KinosailReaderPositions").appendingPathComponent(scope + ".json")
    }
    func pending(itemID: String) throws -> PendingReading? { _ = try Input.id(itemID); try load(); return entries.first { $0.itemID == itemID } }
    func validate(client: ServerClient) async throws { guard try await client.profileScope() == scope else { throw ClientError.http(403) } }
    func record(itemID: String, position: ReaderPosition, expected: ReaderPosition) throws {
        let entry = try PendingReading(itemID: itemID, position: position, expected: expected)
        try load()
        var next = entries
        if let index = next.firstIndex(where: { $0.itemID == itemID }) { next[index].position = entry.position }
        else { guard next.count < 50 else { throw ClientError.invalidInput("Sync saved reading positions before opening more books.") }; next.append(entry) }
        try persist(next)
    }
    func acknowledge(itemID: String, position: ReaderPosition) throws {
        try load()
        var next = entries
        if let index = next.firstIndex(where: { $0.itemID == itemID }) {
            if next[index].position == position { next.remove(at: index) }
            else { next[index].expected = position }
            try persist(next)
        }
    }
    func discard(itemID: String) throws { _ = try Input.id(itemID); try load(); try persist(entries.filter { $0.itemID != itemID }) }
    func rebase(itemID: String, expected: ReaderPosition) throws {
        _ = try ReaderPosition(expected.json); try load()
        var next = entries
        if let index = next.firstIndex(where: { $0.itemID == itemID }) { next[index].expected = expected; try persist(next) }
    }
    private func load() throws {
        guard !loaded else { return }
        if FileManager.default.fileExists(atPath: file.path) {
            let properties = try file.resourceValues(forKeys: [.fileSizeKey, .isRegularFileKey])
            guard file.resolvingSymlinksInPath().standardizedFileURL == file.standardizedFileURL, properties.isRegularFile == true, let size = properties.fileSize, size <= 256 * 1024 else { throw ClientError.invalidResponse }
            let value = try StrictJSON.decode(Data(contentsOf: file), maximum: 256 * 1024).object(allowing: ["version", "entries"])
            guard try value.requiredNumber("version", max: 1, integer: true) == 1 else { throw ClientError.invalidResponse }
            let parsed = try value.required("entries").array(max: 50).map(PendingReading.init)
            guard Set(parsed.map(\.itemID)).count == parsed.count else { throw ClientError.invalidResponse }
            entries = parsed
        }
        loaded = true
    }
    private func persist(_ next: [PendingReading]) throws {
        let data = try JSONEncoder().encode(JSONValue.object(["version": .number(1), "entries": .array(next.map(\.json))]))
        guard data.count <= 256 * 1024 else { throw ClientError.invalidResponse }
        var root = file.deletingLastPathComponent()
        guard root.resolvingSymlinksInPath().standardizedFileURL == root.standardizedFileURL else { throw ClientError.invalidResponse }
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true, attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication])
        var properties = URLResourceValues(); properties.isExcludedFromBackup = true
        try root.setResourceValues(properties)
        try data.write(to: file, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
        entries = next
    }
}

enum ReaderSaveResult: Sendable { case saved, queued, offline, conflict(ReaderPosition) }

actor ReaderProgressWriter {
    private let itemID: String
    private let client: ServerClient
    private let store: ReaderPositionStore
    private var expected: ReaderPosition
    private var pending: ReaderPosition?
    private var sending = false
    private var blocked: ReaderPosition?

    init(itemID: String, client: ServerClient, store: ReaderPositionStore, expected: ReaderPosition) throws {
        self.itemID = try Input.id(itemID); self.client = client; self.store = store
        self.expected = try ReaderPosition(expected.json)
    }
    func update(_ position: ReaderPosition) async throws -> ReaderSaveResult {
        let value = try ReaderPosition(position.json)
        guard value.total == expected.total else { throw ClientError.invalidResponse }
        try await store.validate(client: client)
        try await store.record(itemID: itemID, position: value, expected: expected)
        pending = value
        if let blocked { return .conflict(blocked) }
        return try await flush()
    }
    func chooseDevice(over remote: ReaderPosition) async throws -> ReaderSaveResult {
        expected = try ReaderPosition(remote.json); blocked = nil
        try await store.rebase(itemID: itemID, expected: remote)
        return try await flush()
    }
    func chooseServer(_ remote: ReaderPosition) async throws {
        expected = try ReaderPosition(remote.json); blocked = nil; pending = nil
        try await store.discard(itemID: itemID)
    }
    private func flush() async throws -> ReaderSaveResult {
        guard !sending else { return .queued }
        sending = true; defer { sending = false }
        while let value = pending {
            do {
                let current = try await client.readerPosition(itemID: itemID)
                guard current == expected || current == value else { blocked = current; return .conflict(current) }
                let saved = try await client.saveReaderPosition(itemID: itemID, page: value.page, offset: value.offset)
                try await store.acknowledge(itemID: itemID, position: saved)
                expected = saved
                if pending == value { pending = nil }
            } catch { return .offline }
        }
        return .saved
    }
}
#endif
