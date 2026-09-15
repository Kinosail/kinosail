import Foundation

extension ReaderBook {
    init(_ raw: JSONValue, itemID: String, server: ServerAddress) throws {
        let itemID = try Input.id(itemID)
        let value = try raw.object(allowing: ["id", "title", "type", "pages"])
        id = try Input.id(value.text("id", max: 128, required: true))
        guard id == itemID, let kind = try Kind(rawValue: value.text("type", max: 16, required: true)) else { throw ClientError.invalidResponse }
        self.kind = kind
        title = try value.text("title", max: 512, required: true)
        let pages = try value.required("pages").array(max: 10_000)
        guard !pages.isEmpty, kind != .pdf || pages.count == 1 else { throw ClientError.invalidResponse }
        self.pages = try pages.enumerated().map { index, raw in
            let value = try raw.object(allowing: ["title", "url", "number"])
            let number = Int(try value.requiredNumber("number", max: 10_000, integer: true))
            let url = try ReaderResourcePolicy.remote(value.text("url", max: 2048, required: true), itemID: itemID, server: server)
            guard number == index + 1, kind == .pdf ? url.path == "/read/\(itemID)/file" : url.path.hasPrefix("/read/\(itemID)/asset/") else { throw ClientError.invalidResponse }
            return try Page(number: number, title: value.text("title", max: 512), resource: url)
        }
        guard Set(self.pages.map(\.resource)).count == self.pages.count else { throw ClientError.invalidResponse }
    }
}

extension ReaderPosition {
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["page", "total", "offset"])
        page = Int(try value.requiredNumber("page", max: 10_000, integer: true))
        total = Int(try value.requiredNumber("total", max: 10_000, integer: true))
        offset = try value.number("offset", max: 1)
        guard total >= 1, page >= 1, page <= total else { throw ClientError.invalidResponse }
    }
}

enum ReaderResourcePolicy {
    static func remote(_ raw: String, itemID: String, server: ServerAddress) throws -> URL {
        let id = try Input.id(itemID)
        guard raw.utf8.count <= 2048, !raw.unicodeScalars.contains(where: { CharacterSet.whitespacesAndNewlines.contains($0) }),
              let components = URLComponents(string: raw), components.query == nil, components.fragment == nil,
              components.percentEncodedPath.range(of: "(?:^|/)(?:\\.|%2e){1,2}(?:/|$)", options: [.regularExpression, .caseInsensitive]) == nil else { throw ClientError.invalidResponse }
        let url = try server.mediaURL(raw)
        guard url.path == "/read/\(id)/file" || url.path.hasPrefix("/read/\(id)/asset/") && !url.path.hasSuffix("/") else { throw ClientError.invalidResponse }
        return url
    }

    static func local(_ remote: URL) throws -> URL {
        guard var components = URLComponents(url: remote, resolvingAgainstBaseURL: false) else { throw ClientError.invalidResponse }
        components.scheme = "kinoreader"; components.host = "book"; components.port = nil
        components.user = nil; components.password = nil; components.query = nil; components.fragment = nil
        guard let url = components.url else { throw ClientError.invalidResponse }
        return url
    }
}
