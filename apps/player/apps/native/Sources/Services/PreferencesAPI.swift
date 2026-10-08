import Foundation

extension ServerClient {
    func mediaPreferences(policy: CatalogPolicy = .reload) async throws -> MediaPreferences {
        try await catalog("/api/v1/me/media-preferences", policy: policy, decode: MediaPreferences.init)
    }

    func saveMediaPreferences(_ preferences: MediaPreferences) async throws -> MediaPreferences {
        let validated = try MediaPreferences(preferences.json)
        let saved = try MediaPreferences(await request("/api/v1/me/media-preferences", method: .put, body: validated.json).body)
        await rememberPreferences(saved.json, path: "/api/v1/me/media-preferences")
        return saved
    }

    func playbackPreferences(itemID: String, policy: CatalogPolicy = .reload) async throws -> ItemPlaybackPreferences {
        try await catalog("/api/v1/items/\(Input.id(itemID))/playback-preferences", policy: policy, decode: ItemPlaybackPreferences.init)
    }

    func playbackPreparationPreferences(itemID: String) async throws -> ItemPlaybackPreferences {
        // A previous playback's checkpoint may invalidate browse caches during startup.
        let path = "/api/v1/items/\(try Input.id(itemID))/playback-preferences"
        do { return try ItemPlaybackPreferences(await request(path).body) }
        catch {
            if error as? ClientError == .http(401) || error as? ClientError == .http(403) { await discardMediaCache() }
            else if error as? ClientError == .http(404), let store = try? cacheStore() { await store.remove(path, kind: .catalog) }
            throw error
        }
    }

    func savePlaybackPreferences(itemID: String, preferences: PlaybackPreferences) async throws -> ItemPlaybackPreferences {
        let validated = try PlaybackPreferences(preferences.json)
        let path = "/api/v1/items/\(try Input.id(itemID))/playback-preferences"
        let raw = try await request(path, method: .put, body: validated.json).body
        let saved = try ItemPlaybackPreferences(raw)
        await rememberPreferences(raw, path: path)
        return saved
    }

    func resetPlaybackPreferences(itemID: String) async throws -> ItemPlaybackPreferences {
        let path = "/api/v1/items/\(try Input.id(itemID))/playback-preferences"
        let raw = try await request(path, method: .delete).body
        let saved = try ItemPlaybackPreferences(raw)
        await rememberPreferences(raw, path: path)
        return saved
    }

    private func rememberPreferences(_ value: JSONValue, path: String) async {
        catalogRequests[path]?.task.cancel()
        catalogRequests[path] = nil
        if let store = try? cacheStore(), let data = try? JSONEncoder().encode(value) {
            await store.remove(path, kind: .catalog)
            await store.enqueueWrite(data, key: path, kind: .catalog)
        }
    }
}
