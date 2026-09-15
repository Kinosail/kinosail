import Foundation

extension ServerClient {
    func mediaPreferences() async throws -> MediaPreferences {
        try MediaPreferences(await request("/api/v1/me/media-preferences").body)
    }

    func saveMediaPreferences(_ preferences: MediaPreferences) async throws -> MediaPreferences {
        let validated = try MediaPreferences(preferences.json)
        return try MediaPreferences(await request("/api/v1/me/media-preferences", method: .put, body: validated.json).body)
    }

    func playbackPreferences(itemID: String) async throws -> ItemPlaybackPreferences {
        try ItemPlaybackPreferences(await request("/api/v1/items/\(Input.id(itemID))/playback-preferences").body)
    }

    func savePlaybackPreferences(itemID: String, preferences: PlaybackPreferences) async throws -> ItemPlaybackPreferences {
        let validated = try PlaybackPreferences(preferences.json)
        return try ItemPlaybackPreferences(await request("/api/v1/items/\(Input.id(itemID))/playback-preferences", method: .put, body: validated.json).body)
    }

    func resetPlaybackPreferences(itemID: String) async throws -> ItemPlaybackPreferences {
        try ItemPlaybackPreferences(await request("/api/v1/items/\(Input.id(itemID))/playback-preferences", method: .delete).body)
    }
}
