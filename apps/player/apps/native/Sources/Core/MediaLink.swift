import Foundation

/// System surfaces carry identifiers only. The active session authorizes content again.
struct MediaLink: Hashable, Identifiable, Sendable {
    enum Action: String, Sendable { case play, detail, search }
    let action: Action
    let value: String
    let scope: String
    var id: String { "\(action.rawValue):\(scope):\(value)" }

    init(action: Action, value: String, scope: String) throws {
        self.action = action
        self.scope = try Input.hex(scope, count: 64)
        self.value = try action == .search ? Input.text(value, max: 512, label: "search", empty: false) : Input.id(value)
    }

    init(url: URL) throws {
        guard url.absoluteString.utf8.count <= 4096,
              let parts = URLComponents(url: url, resolvingAgainstBaseURL: false),
              parts.scheme == "kinosail", parts.host == "media", parts.path.isEmpty,
              parts.user == nil, parts.password == nil, parts.port == nil, parts.fragment == nil,
              let query = parts.queryItems, query.count == 3,
              Set(query.map(\.name)) == ["action", "value", "scope"],
              let raw = query.first(where: { $0.name == "action" })?.value, let action = Action(rawValue: raw),
              let value = query.first(where: { $0.name == "value" })?.value,
              let scope = query.first(where: { $0.name == "scope" })?.value else {
            throw ClientError.invalidInput("That library link is invalid.")
        }
        try self.init(action: action, value: value, scope: scope)
    }

    var url: URL {
        var parts = URLComponents()
        parts.scheme = "kinosail"; parts.host = "media"
        parts.queryItems = [URLQueryItem(name: "action", value: action.rawValue),
                            URLQueryItem(name: "value", value: value), URLQueryItem(name: "scope", value: scope)]
        return parts.url!
    }
}
