import Foundation

extension ServerClient {
    func showListed(showID: String) async throws -> Bool {
        let id = try Input.hex(showID, count: 16)
        return try await request("/api/v1/shows/\(id)/list").body.object(allowing: ["listed"]).flag("listed")
    }

    func setShowListed(showID: String, listed: Bool) async throws -> Bool {
        let id = try Input.hex(showID, count: 16)
        let saved = try await request("/api/v1/shows/\(id)/list", method: .put, body: .object(["listed": .bool(listed)]))
            .body.object(allowing: ["listed"]).flag("listed")
        guard saved == listed else { throw ClientError.invalidResponse }
        await invalidateCatalog()
        return saved
    }

    func listedShows(policy: CatalogPolicy = .reload) async throws -> [MediaItem] {
        try await catalog("/api/v1/shows?listed=true", policy: policy) { [self] raw in
            try raw.object(allowing: ["shows"]).list("shows", max: 2000).map { entry in
                let show = try entry.object(allowing: ["id", "title", "artwork", "backdrop", "artist", "play"])
                var item: [String: JSONValue] = ["id": .string(try Input.hex(show.text("id", max: 16, required: true), count: 16)),
                                                 "kind": .string("show"), "title": .string(try show.text("title", max: 512))]
                for key in ["artwork", "backdrop"] where show[key] != nil { item[key] = .string(try show.text(key, max: 2048)) }
                return try MediaItem(.object(item), server: server)
            }
        }
    }
}
