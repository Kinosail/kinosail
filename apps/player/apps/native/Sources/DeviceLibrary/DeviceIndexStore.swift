import Foundation

actor DeviceIndexStore {
    private static let limit = 60_000
    private let file: URL?
    private var entries: [String: DeviceItem] = [:]
    private var loaded = false
    private var pending: Task<Void, Never>?

    init(directory: URL? = nil) {
        let base = directory ?? FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first
        file = base?.appendingPathComponent("DeviceLibrary", isDirectory: true).appendingPathComponent("index.json")
    }

    func item(_ id: String) -> DeviceItem? {
        load()
        return entries[id]
    }

    func store(_ items: [DeviceItem]) {
        load()
        for item in items where !item.id.isEmpty { entries[item.id] = item }
        if entries.count > Self.limit {
            let keep = entries.values.sorted { ($0.state.lastPlayed ?? $0.added ?? .distantPast) > ($1.state.lastPlayed ?? $1.added ?? .distantPast) }.prefix(Self.limit)
            entries = Dictionary(uniqueKeysWithValues: keep.map { ($0.id, $0) })
        }
        schedule()
    }

    func clear() {
        entries = [:]; loaded = true
        pending?.cancel(); pending = nil
        if let file { try? FileManager.default.removeItem(at: file) }
    }

    private func load() {
        guard !loaded else { return }
        loaded = true
        guard let file, let data = try? Data(contentsOf: file), data.count <= 256 * 1024 * 1024,
              let saved = try? JSONDecoder().decode([String: DeviceItem].self, from: data) else { return }
        entries = saved
    }

    private func schedule() {
        guard pending == nil else { return }
        pending = Task { [weak self] in
            try? await Task.sleep(for: .seconds(2))
            await self?.flush()
        }
    }

    private func flush() {
        pending = nil
        guard let file, let data = try? JSONEncoder().encode(entries) else { return }
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? data.write(to: file, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
    }
}
