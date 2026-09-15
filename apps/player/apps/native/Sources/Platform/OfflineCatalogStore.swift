#if os(iOS)
import Foundation

struct OfflineRecord: Identifiable, Sendable {
    let key: String
    let jobID: String
    let item: MediaItem
    let quality: DownloadQuality
    let tracks: DownloadTrackSelection?
    var error = ""
    var deleting = false
    var id: String { key }

    init(item: MediaItem, jobID: String, quality: DownloadQuality, tracks: DownloadTrackSelection?) throws {
        self.jobID = try Input.hex(jobID, count: 16)
        self.item = item
        self.quality = quality
        self.tracks = try tracks?.validated()
        let selection = tracks.map { $0.audio.sorted().map(String.init).joined(separator: ",") + ":" + $0.subtitles.sorted().map(String.init).joined(separator: ",") } ?? "all"
        key = DownloadAuthorization.hash(Data("\(item.id)\n\(quality.rawValue)\n\(selection)".utf8))
    }

    init(_ raw: JSONValue, server: ServerAddress) throws {
        let value = try raw.object(allowing: ["key", "jobID", "item", "quality", "tracks", "error", "deleting"])
        let item = try MediaItem(value.required("item"), server: server)
        guard let quality = try DownloadQuality(rawValue: value.text("quality", max: 32, required: true)) else { throw ClientError.invalidResponse }
        let tracks = try value["tracks"].map(DownloadTrackSelection.init)
        try OfflineDownloadManager.validate(item: item, quality: quality, tracks: tracks)
        try self.init(item: item, jobID: value.text("jobID", max: 16, required: true), quality: quality, tracks: tracks)
        guard try value.text("key", max: 64, required: true) == key else { throw ClientError.invalidResponse }
        error = try value.text("error", max: 512)
        deleting = try value.flag("deleting", fallback: false)
    }

    var json: JSONValue {
        var value: [String: JSONValue] = ["key": .string(key), "jobID": .string(jobID), "item": item.json, "quality": .string(quality.rawValue), "error": .string(error), "deleting": .bool(deleting)]
        if let tracks { value["tracks"] = tracks.json }
        return .object(value)
    }
}

struct OfflineCatalog: Sendable {
    var records: [OfflineRecord] = []
    var preferences = MediaPreferences()
}

struct OfflineCompletion: Sendable {
    let item: MediaItem
    let nextID: String?
    let quality: DownloadQuality
    var json: JSONValue {
        var value: [String: JSONValue] = ["item": item.json, "quality": .string(quality.rawValue)]
        if let nextID { value["nextID"] = .string(nextID) }
        return .object(value)
    }
    init(item: MediaItem, nextID: String?, quality: DownloadQuality) throws {
        self.item = item; self.nextID = try nextID.map(Input.id); self.quality = quality
    }
    init(_ raw: JSONValue, server: ServerAddress) throws {
        let value = try raw.object(allowing: ["item", "nextID", "quality"])
        guard let quality = try DownloadQuality(rawValue: value.text("quality", max: 32, required: true)) else { throw ClientError.invalidResponse }
        try self.init(item: MediaItem(value.required("item"), server: server), nextID: value["nextID"] == nil ? nil : value.text("nextID", max: 128, required: true), quality: quality)
    }
}

actor OfflineCatalogStore {
    private let file: URL
    private let server: ServerAddress
    static var root: URL { FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].resolvingSymlinksInPath().appendingPathComponent("KinosailOfflineCatalog", isDirectory: true) }

    init(scope: String, server: ServerAddress) throws {
        file = Self.root.appendingPathComponent(try Input.hex(scope, count: 64) + ".json")
        self.server = server
    }
    func load() throws -> OfflineCatalog {
        guard FileManager.default.fileExists(atPath: file.path) else { return OfflineCatalog() }
        let properties = try file.resourceValues(forKeys: [.fileSizeKey, .isRegularFileKey])
        guard file.resolvingSymlinksInPath().standardizedFileURL == file.standardizedFileURL, properties.isRegularFile == true,
              let size = properties.fileSize, size <= 2 * 1024 * 1024 else { throw ClientError.invalidResponse }
        let value = try StrictJSON.decode(Data(contentsOf: file)).object(allowing: ["version", "records", "preferences"])
        guard try value.requiredNumber("version", max: 1, integer: true) == 1 else { throw ClientError.invalidResponse }
        return try OfflineCatalog(records: Input.unique(value.required("records").array(max: 50).map { try OfflineRecord($0, server: server) }), preferences: MediaPreferences(value.required("preferences")))
    }
    func save(_ catalog: OfflineCatalog) throws {
        guard catalog.records.count <= 50, Set(catalog.records.map(\.key)).count == catalog.records.count else { throw ClientError.invalidResponse }
        let value = JSONValue.object(["version": .number(1), "records": .array(catalog.records.map(\.json)), "preferences": catalog.preferences.json])
        for record in catalog.records { _ = try OfflineRecord(record.json, server: server) }
        _ = try MediaPreferences(catalog.preferences.json)
        let data = try JSONEncoder().encode(value)
        guard data.count <= 2 * 1024 * 1024, Self.root.resolvingSymlinksInPath().standardizedFileURL == Self.root.standardizedFileURL else { throw ClientError.invalidResponse }
        try FileManager.default.createDirectory(at: Self.root, withIntermediateDirectories: true, attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication])
        var root = Self.root
        var properties = URLResourceValues(); properties.isExcludedFromBackup = true
        try root.setResourceValues(properties)
        try data.write(to: file, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
    }
    private var completionFile: URL { file.deletingPathExtension().appendingPathExtension("completed.json") }
    func completions() throws -> [OfflineCompletion] {
        guard FileManager.default.fileExists(atPath: completionFile.path) else { return [] }
        let values = try completionFile.resourceValues(forKeys: [.fileSizeKey, .isRegularFileKey])
        guard completionFile.resolvingSymlinksInPath().standardizedFileURL == completionFile.standardizedFileURL,
              values.isRegularFile == true, let size = values.fileSize, size <= 2 * 1024 * 1024 else { throw ClientError.invalidResponse }
        let pending = try StrictJSON.decode(Data(contentsOf: completionFile)).array(max: 50).map { try OfflineCompletion($0, server: server) }
        guard Set(pending.map { $0.item.id }).count == pending.count else { throw ClientError.invalidResponse }
        return pending
    }
    func recordCompletion(_ completion: OfflineCompletion) throws -> [OfflineCompletion] {
        var pending = try completions()
        if pending.contains(where: { $0.item.id == completion.item.id }) { return pending }
        guard pending.count < 50 else { throw ClientError.invalidInput("Sync automatic downloads before finishing more titles.") }
        _ = try OfflineCompletion(completion.json, server: server)
        pending.append(completion)
        try saveCompletions(pending)
        return pending
    }
    func removeCompletion(_ itemID: String) throws -> [OfflineCompletion] {
        var pending = try completions()
        pending.removeAll { $0.item.id == itemID }
        try saveCompletions(pending)
        return pending
    }
    private func saveCompletions(_ pending: [OfflineCompletion]) throws {
        let data = try JSONEncoder().encode(JSONValue.array(pending.map(\.json)))
        guard data.count <= 2 * 1024 * 1024, completionFile.resolvingSymlinksInPath().standardizedFileURL == completionFile.standardizedFileURL else { throw ClientError.invalidResponse }
        try FileManager.default.createDirectory(at: Self.root, withIntermediateDirectories: true, attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication])
        try data.write(to: completionFile, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
    }
    static func reset() throws {
        guard root.resolvingSymlinksInPath().standardizedFileURL == root.standardizedFileURL else { throw ClientError.invalidResponse }
        if FileManager.default.fileExists(atPath: root.path) { try FileManager.default.removeItem(at: root) }
    }
}
#endif
