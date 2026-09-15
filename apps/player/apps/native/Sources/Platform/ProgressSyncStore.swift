import Foundation

struct PendingProgress: Identifiable, Sendable {
    let itemID: String
    var progress: WatchProgress
    var expected: WatchProgress
    var conflict: WatchProgress?
    var id: String { itemID }

    init(itemID: String, progress: WatchProgress, expected: WatchProgress, conflict: WatchProgress? = nil) throws {
        self.itemID = try Input.id(itemID)
        self.progress = try progress.validated(required: true)
        self.expected = try expected.validated(required: false)
        self.conflict = try conflict?.validated(required: false)
    }

    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["itemID", "progress", "expected", "conflict"])
        try self.init(itemID: value.text("itemID", max: 128, required: true), progress: WatchProgress(value.required("progress")),
                      expected: WatchProgress(value.required("expected")), conflict: value["conflict"].map(WatchProgress.init))
    }

    var json: JSONValue {
        var value: [String: JSONValue] = ["itemID": .string(itemID), "progress": progress.json, "expected": expected.json]
        if let conflict { value["conflict"] = conflict.json }
        return .object(value)
    }
}

/// Canonical source positions only. Tokens and account credentials never enter
/// this file. The scope includes Server origin, Server identity and Viewer Profile.
actor ProgressSyncStore {
    private let file: URL
    private let scope: String
    private var entries: [PendingProgress] = []
    private var loaded = false
    private var syncing = false

    init(scope: String, directory: URL? = nil) throws {
        self.scope = try Input.hex(scope, count: 64)
        let root = directory ?? FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].resolvingSymlinksInPath()
            .appendingPathComponent("KinosailProgress", isDirectory: true)
        file = root.appendingPathComponent(scope + ".json")
    }

    func pending() throws -> [PendingProgress] { try load(); return entries }

    func record(itemID: String, progress: WatchProgress, expected: WatchProgress) throws {
        let entry = try PendingProgress(itemID: itemID, progress: progress, expected: expected)
        try load()
        var next = entries
        if let index = next.firstIndex(where: { $0.itemID == entry.itemID }) {
            if next[index].progress.session == entry.progress.session, next[index].progress.revision >= entry.progress.revision { return }
            next[index].progress = entry.progress
        } else {
            guard next.count < 50 else { throw ClientError.invalidInput("Open Progress sync to save your pending positions before starting more titles.") }
            next.append(entry)
        }
        try persist(next)
    }

    func synchronize(client: ServerClient) async throws -> [String: ProgressSyncResult] {
        guard try await client.profileScope() == scope else { throw ClientError.http(403) }
        try load()
        guard !syncing else { return [:] }
        syncing = true
        defer { syncing = false }
        var results: [String: ProgressSyncResult] = [:]
        for entry in entries where entry.conflict == nil {
            try Task.checkCancellation()
            let result: ProgressSyncResult
            do { result = try await client.syncProgress(itemID: entry.itemID, progress: entry.progress, expected: entry.expected, playbackToken: "") }
            catch ClientError.http(404) { continue }
            catch ClientError.http(410) { continue }
            // Keep unavailable titles on disk, but let other titles synchronize.
            // Authentication, transport and persistence errors still stop this attempt.
            guard let index = entries.firstIndex(where: { $0.itemID == entry.itemID }) else { continue }
            var next = entries
            if result.conflict { next[index].conflict = result.progress }
            else if next[index].progress == entry.progress { next.remove(at: index) }
            else { next[index].expected = result.progress }
            try persist(next)
            results[entry.itemID] = result
        }
        return results
    }

    func resolve(itemID: String, useDevice: Bool) throws {
        let id = try Input.id(itemID)
        try load()
        guard let index = entries.firstIndex(where: { $0.itemID == id }), let remote = entries[index].conflict else {
            throw ClientError.invalidInput("These saved positions have changed. Reopen Progress sync.")
        }
        var next = entries
        if useDevice { next[index].expected = remote; next[index].conflict = nil }
        else { next.remove(at: index) }
        try persist(next)
    }

    private func load() throws {
        guard !loaded else { return }
        if FileManager.default.fileExists(atPath: file.path) {
            let values = try file.resourceValues(forKeys: [.fileSizeKey, .isRegularFileKey, .isSymbolicLinkKey])
            guard values.isRegularFile == true, values.isSymbolicLink != true, let size = values.fileSize, size <= 256 * 1024 else { throw ClientError.invalidResponse }
            let root = try StrictJSON.decode(Data(contentsOf: file), maximum: 256 * 1024).object(allowing: ["version", "entries"])
            guard try root.requiredNumber("version", max: 1, integer: true) == 1 else { throw ClientError.invalidResponse }
            entries = try Input.unique(root.required("entries").array(max: 50).map(PendingProgress.init))
        }
        loaded = true
    }

    private func persist(_ next: [PendingProgress]) throws {
        let data = try JSONEncoder().encode(JSONValue.object(["version": .number(1), "entries": .array(next.map(\.json))]))
        guard data.count <= 256 * 1024 else { throw ClientError.invalidResponse }
        let root = file.deletingLastPathComponent()
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        var directory = root
        var values = URLResourceValues()
        values.isExcludedFromBackup = true
        try directory.setResourceValues(values)
        try data.write(to: file, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
        entries = next
    }
}
