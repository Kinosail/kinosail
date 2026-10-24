import Foundation

struct QuickConnectLink: Identifiable, Hashable, Sendable {
    let serverID: String
    let code: String

    var id: String { "\(serverID):\(code)" }

    init(serverID: String, code: String) throws {
        guard serverID.range(of: "^[A-Za-z0-9-]{1,128}$", options: .regularExpression) != nil else {
            throw ClientError.invalidInput("That TV sign-in link is invalid.")
        }
        guard code.range(of: "^[0-9]{4,12}$", options: .regularExpression) != nil else {
            throw ClientError.invalidInput("That TV sign-in code is invalid.")
        }
        self.serverID = serverID
        self.code = code
    }

    init?(_ url: URL) throws {
        guard let parts = URLComponents(url: url, resolvingAgainstBaseURL: false),
              parts.scheme == "kinosail", parts.host == "quickconnect" else { return nil }
        guard url.absoluteString.utf8.count <= 1024, parts.path.isEmpty, parts.user == nil, parts.password == nil,
              parts.port == nil, parts.fragment == nil, let query = parts.queryItems, query.count == 2,
              Set(query.map(\.name)) == ["server", "code"],
              let server = query.first(where: { $0.name == "server" })?.value,
              let code = query.first(where: { $0.name == "code" })?.value else {
            throw ClientError.invalidInput("That TV sign-in link is invalid.")
        }
        try self.init(serverID: server, code: code)
    }

    var url: URL? {
        var parts = URLComponents()
        parts.scheme = "kinosail"
        parts.host = "quickconnect"
        parts.queryItems = [URLQueryItem(name: "server", value: serverID), URLQueryItem(name: "code", value: code)]
        return parts.url
    }
}
