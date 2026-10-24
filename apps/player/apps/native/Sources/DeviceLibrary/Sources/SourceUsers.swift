import Foundation

struct SourcePublicUser: Identifiable, Hashable, Sendable {
    let id: String
    let name: String
    let hasPassword: Bool
    let hasImage: Bool
}

struct SourcePreview: Sendable {
    let movies: Int
    let shows: Int
    let posters: [Data]
}

extension SourceConnect {
    static func publicUsers(_ server: SourceServerInfo) async -> [SourcePublicUser] {
        guard let request = try? JellyfinClient.request(SourceHTTP.url(server.address, "/Users/Public"), token: nil),
              let data = try? await SourceHTTP.data(request, maximum: 512 * 1024, expected: [200]),
              let rows = try? JSONSerialization.jsonObject(with: data) as? [[String: Any]] else { return [] }
        return rows.prefix(24).compactMap { row in
            guard let id = SourceHTTP.identifier(row["Id"]) else { return nil }
            let name = SourceHTTP.text(row["Name"], max: 120)
            guard !name.isEmpty else { return nil }
            return SourcePublicUser(id: id, name: name, hasPassword: row["HasPassword"] as? Bool ?? true,
                                    hasImage: SourceHTTP.identifier(row["PrimaryImageTag"]) != nil)
        }
    }

    static func avatar(_ server: SourceServerInfo, user: SourcePublicUser) async -> Data? {
        guard user.hasImage, let request = try? JellyfinClient.request(
            SourceHTTP.url(server.address, "/Users/\(user.id)/Images/Primary", ["maxWidth": "240", "quality": "90"]), token: nil) else { return nil }
        return try? await SourceHTTP.data(request, maximum: 4 * 1024 * 1024, expected: [200])
    }

    static func preview(_ account: SourceAccount) async throws -> SourcePreview {
        let client = DeviceCatalog.client(for: account)
        let items = try await client.topLevel()
        let recent = items.filter(\.hasPrimary).sorted { ($0.added ?? .distantPast) > ($1.added ?? .distantPast) }.prefix(5)
        var posters: [Data] = []
        for item in recent {
            guard let request = try? client.image(item.ref, kind: .primary, width: 360),
                  let data = try? await SourceHTTP.data(request, maximum: 4 * 1024 * 1024, expected: [200]) else { continue }
            posters.append(data)
        }
        return SourcePreview(movies: items.filter { $0.kind == .movie }.count, shows: items.filter { $0.kind == .series }.count, posters: posters)
    }
}
