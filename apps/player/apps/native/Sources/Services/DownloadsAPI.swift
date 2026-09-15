import Foundation

extension ServerClient {
    func prepareDownload(itemID: String, quality: DownloadQuality, tracks: DownloadTrackSelection? = nil) async throws -> PreparedDownload {
        let id = try Input.id(itemID)
        let tracks = try tracks?.validated()
        guard quality != .original || tracks == nil else { throw ClientError.invalidInput("Original downloads include all audio and subtitle tracks in the file.") }
        var body: [String: JSONValue] = ["quality": .string(quality.rawValue)]
        if let tracks { body["tracks"] = tracks.json }
        let result = try PreparedDownload(await request("/api/v1/items/\(id)/downloads", method: .post, body: .object(body), expected: [202]).body, profileID: viewerID)
        guard result.itemID == id, result.quality == quality else { throw ClientError.invalidResponse }
        return result
    }

    func preparedDownload(id: String) async throws -> PreparedDownload {
        let id = try Input.hex(id, count: 16)
        let result = try PreparedDownload(await request("/api/v1/downloads/\(id)").body, profileID: viewerID)
        guard result.id == id else { throw ClientError.invalidResponse }
        return result
    }

    func downloadManifest(id: String) async throws -> OfflineManifest {
        let id = try Input.hex(id, count: 16)
        let result = try OfflineManifest(await request("/api/v1/downloads/\(id)/manifest").body)
        guard result.id == id else { throw ClientError.invalidResponse }
        return result
    }

    func downloadTracks(itemID: String) async throws -> DownloadTrackOptions {
        let value = try await request("/api/v1/items/\(Input.id(itemID))/download-tracks").body.object(allowing: ["audio", "subtitles"])
        func tracks(_ key: String, maximum: Int) throws -> [DownloadTrackOptions.Track] {
            try Input.unique(value.required(key).array(max: maximum).map { raw in
                let track = try raw.object(allowing: ["index", "label"])
                return try DownloadTrackOptions.Track(index: Int(track.requiredNumber("index", max: 4095, integer: true)), label: track.text("label", max: 256, required: true))
            })
        }
        return try DownloadTrackOptions(audio: tracks("audio", maximum: 32), subtitles: tracks("subtitles", maximum: 256))
    }

    func downloadIdentity() async throws -> DownloadIdentity {
        let value = try await request("/api/v1/downloads/identity").body.object(allowing: ["serverId", "profileId"])
        let profile = try Input.id(value.text("profileId", max: 128, required: true))
        let server = try value.text("serverId", max: 256, required: true)
        let identity = try downloadAuthorization()
        let returned = try DownloadAuthorization(server: self.server, serverID: server, profileID: profile, token: "identity-check")
        guard returned.scope == identity.scope else { throw ClientError.invalidResponse }
        return DownloadIdentity(serverID: server, profileID: profile)
    }
}
