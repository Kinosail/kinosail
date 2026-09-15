import Foundation

struct CastMember: Codable, Hashable, Sendable {
    let name: String
    let role: String
    let image: String

    init(_ raw: JSONValue, server: ServerAddress) throws {
        let value = try raw.object(allowing: ["name", "role", "image"])
        name = try value.text("name", max: 256, required: true)
        role = try value.text("role", max: 512)
        image = try value.text("image")
        if !image.isEmpty { _ = try server.mediaURL(image) }
    }

    static func list(_ value: [String: JSONValue], server: ServerAddress) throws -> [Self] {
        try value.list("cast", max: 256).map { try Self($0, server: server) }
    }

    var json: JSONValue { .object(["name": .string(name), "role": .string(role), "image": .string(image)]) }
}

struct ShowDetail: Sendable {
    let episodes: [MediaItem]
    let cast: [CastMember]
}

extension Input {
    static func actorName(_ raw: String) throws -> String {
        let name = try text(raw, max: 200, label: "actor name")
            .split(whereSeparator: \.isWhitespace).joined(separator: " ").precomposedStringWithCanonicalMapping
        return try text(name, max: 200, label: "actor name")
    }
}

struct ActorDetail: Sendable {
    struct Credit: Identifiable, Sendable {
        let id: String
        let title: String
        let year: String
        let role: String
        let artwork: String
    }
    let name: String
    let image: String
    let movies: [Credit]
    let shows: [Credit]

    init(_ raw: JSONValue, name expected: String, server: ServerAddress) throws {
        let value = try raw.object(allowing: ["name", "image", "movies", "shows"])
        name = try value.text("name", max: 200, required: true)
        guard name == expected else { throw ClientError.invalidResponse }
        image = try value.text("image")
        if !image.isEmpty { _ = try server.mediaURL(image) }
        func credits(_ key: String) throws -> [Credit] {
            try Input.unique(value.required(key).array(max: 10_000).map { raw in
                let credit = try raw.object(allowing: ["id", "title", "year", "role", "artwork", "url"])
                let id = try Input.id(credit.text("id", max: 128, required: true))
                if key == "shows" { _ = try Input.hex(id, count: 16) }
                let route = key == "shows" ? "/show/" : "/watch/"
                guard try credit.text("url", required: true) == route + id else { throw ClientError.invalidResponse }
                let artwork = try credit.text("artwork")
                if !artwork.isEmpty { _ = try server.mediaURL(artwork) }
                // A show credit joins roles across episodes; the bounded response is its size ceiling.
                return try Credit(id: id, title: credit.text("title", max: 512, required: true),
                                  year: credit.text("year", max: 16), role: credit.text("role", max: 2 * 1024 * 1024), artwork: artwork)
            })
        }
        movies = try credits("movies")
        shows = try credits("shows")
    }
}
