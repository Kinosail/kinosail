import Foundation

// A link can select a request to review. It cannot change Server or approve it.
enum ApprovalLink {
    static func code(_ url: URL, server: ServerAddress) throws -> String {
        guard url.absoluteString.utf8.count <= 4096,
              let parts = URLComponents(url: url, resolvingAgainstBaseURL: false),
              parts.scheme == "kinosail", parts.host == "approve", parts.path.isEmpty,
              parts.user == nil, parts.password == nil, parts.port == nil, parts.fragment == nil,
              let query = parts.queryItems, query.count == 2,
              Set(query.map(\.name)) == ["server", "code"],
              let origin = query.first(where: { $0.name == "server" })?.value,
              let code = query.first(where: { $0.name == "code" })?.value else { throw ClientError.invalidInput("That TV sign-in link is invalid.") }
        guard try ServerAddress(origin) == server else {
            throw ClientError.invalidInput("That TV uses a different Server. Approve it in the Player connected to that Server.")
        }
        return try Input.code(code)
    }

    static func url(server: ServerAddress, code: String) throws -> URL {
        try server.mediaURL("/connect?code=\(Input.code(code))")
    }
}
