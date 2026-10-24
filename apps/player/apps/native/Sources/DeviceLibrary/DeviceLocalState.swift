import Foundation

struct ProgressMark: Codable, Hashable, Sendable {
    var session = ""
    var revision = 0
}

actor DeviceLocalState {
    private struct Saved: Codable {
        var media: Data?
        var items: [String: Data] = [:]
        var marks: [String: ProgressMark] = [:]
    }

    struct Playback: Sendable {
        let ref: SourceRef
        let session: String
        let direct: URLRequest?
        let subtitles: [Int: URLRequest]
        var started = false
        var seconds: Double = 0
    }

    private let file: URL?
    private var saved = Saved()
    private var loaded = false
    private var playbacks: [String: Playback] = [:]

    init(directory: URL? = nil) {
        let base = directory ?? FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
        file = base?.appendingPathComponent("DeviceLibrary", isDirectory: true).appendingPathComponent("state.json")
    }

    func mediaPreferences() -> JSONValue {
        load()
        if let data = saved.media, let value = try? StrictJSON.decode(data, maximum: 16 * 1024), (try? MediaPreferences(value)) != nil { return value }
        return Self.defaults.json
    }

    func saveMediaPreferences(_ value: JSONValue) throws -> JSONValue {
        let preferences = try MediaPreferences(value)
        load()
        saved.media = try JSONEncoder().encode(preferences.json)
        persist()
        return preferences.json
    }

    func itemPreferences(_ id: String) -> JSONValue {
        load()
        if let data = saved.items[id], let value = try? StrictJSON.decode(data, maximum: 16 * 1024), (try? PlaybackPreferences(value)) != nil {
            return .object(["playback": value, "overridden": .bool(true)])
        }
        let fallback = (try? MediaPreferences(mediaPreferences()))?.playback ?? PlaybackPreferences()
        return .object(["playback": fallback.json, "overridden": .bool(false)])
    }

    func saveItemPreferences(_ id: String, _ value: JSONValue?) throws -> JSONValue {
        load()
        if let value {
            let preferences = try PlaybackPreferences(value)
            saved.items[id] = try JSONEncoder().encode(preferences.json)
            if saved.items.count > 5_000, let first = saved.items.keys.sorted().first { saved.items.removeValue(forKey: first) }
        } else { saved.items.removeValue(forKey: id) }
        persist()
        return itemPreferences(id)
    }

    func mark(_ id: String) -> ProgressMark {
        load()
        return saved.marks[id] ?? ProgressMark()
    }

    func marks(_ ids: [String]) -> [String: ProgressMark] {
        load()
        return saved.marks.filter { ids.contains($0.key) }
    }

    func record(_ id: String, _ mark: ProgressMark) {
        load()
        saved.marks[id] = mark
        if saved.marks.count > 20_000, let first = saved.marks.keys.sorted().first { saved.marks.removeValue(forKey: first) }
        persist()
    }

    func begin(_ id: String, _ playback: Playback) {
        playbacks[id] = playback
        if playbacks.count > 16, let first = playbacks.keys.first(where: { $0 != id }) { playbacks.removeValue(forKey: first) }
    }

    func playback(_ id: String) -> Playback? { playbacks[id] }

    func reported(_ id: String, seconds: Double) {
        guard var playback = playbacks[id] else { return }
        playback.started = true
        playback.seconds = seconds
        playbacks[id] = playback
    }

    func end(_ id: String) -> Playback? { playbacks.removeValue(forKey: id) }

    func clear() {
        saved = Saved(); loaded = true; playbacks = [:]
        if let file { try? FileManager.default.removeItem(at: file) }
    }

    static var defaults: MediaPreferences {
        var preferences = MediaPreferences()
        preferences.downloadLimitGiB = 20
        return preferences
    }

    private func load() {
        guard !loaded else { return }
        loaded = true
        guard let file, let data = try? Data(contentsOf: file), data.count <= 16 * 1024 * 1024,
              let value = try? JSONDecoder().decode(Saved.self, from: data) else { return }
        saved = value
    }

    private func persist() {
        guard let file, let data = try? JSONEncoder().encode(saved) else { return }
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? data.write(to: file, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
    }
}
