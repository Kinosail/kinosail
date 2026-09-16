import Foundation

extension ServerClient {
    func playback(itemID: String) async throws -> PlaybackSource {
        let id = try Input.id(itemID)
        let capabilities = try await PlaybackCapabilities.detect()
        return try PlaybackSource(await request("/api/v1/items/\(id)/playback?\(capabilities.query)").body, itemID: id, server: server)
    }

    func syncProgress(itemID: String, progress: WatchProgress, expected: WatchProgress, playbackToken: String) async throws -> ProgressSyncResult {
        let valid = try progress.validated(required: true)
        let baseline = try expected.validated(required: false)
        let token = try Input.text(playbackToken, max: 8192, label: "playback token", empty: true)
        let response = try await request("/api/v1/items/\(Input.id(itemID))/progress/sync", method: .put,
                                         body: .object(["progress": valid.json, "expected": baseline.json, "playbackToken": .string(token)]), expected: [200, 409])
        if response.status == 409 {
            let value = try response.body.object(allowing: ["error", "progress"])
            _ = try value.text("error", max: 512)
            let result = try ProgressSyncResult(progress: WatchProgress(value.required("progress")), conflict: true)
            await invalidateCatalog()
            return result
        }
        let result = try ProgressSyncResult(progress: WatchProgress(response.body), conflict: false)
        await invalidateCatalog()
        return result
    }

    func bookmarks(itemID: String) async throws -> [Bookmark] {
        try parseBookmarks(await request("/api/v1/items/\(Input.id(itemID))/bookmarks?includeOffset=true").body)
    }

    func addBookmark(itemID: String, title: String, position: BookmarkPosition) async throws -> [Bookmark] {
        let body = try Bookmark.request(title: title, position: position)
        return try parseBookmarks(await request("/api/v1/items/\(Input.id(itemID))/bookmarks?includeOffset=true", method: .post, body: body, expected: [201]).body)
    }

    func removeBookmark(itemID: String, bookmarkID: String) async throws -> [Bookmark] {
        try parseBookmarks(await request("/api/v1/items/\(Input.id(itemID))/bookmarks/\(Input.hex(bookmarkID, count: 64))?includeOffset=true", method: .delete).body)
    }

    private func parseBookmarks(_ raw: JSONValue) throws -> [Bookmark] {
        let value = try raw.object(allowing: ["bookmarks"])
        return try Input.unique(value.required("bookmarks").array(max: 100).map(Bookmark.init))
    }
}
