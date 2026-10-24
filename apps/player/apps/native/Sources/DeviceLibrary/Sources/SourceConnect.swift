import Foundation

struct SourceServerInfo: Identifiable, Hashable, Sendable {
    let kind: SourceKind
    let serverID: String
    let name: String
    let version: String
    let address: URL

    var id: String { "\(kind.rawValue):\(serverID)" }
}

struct SourceQuickConnect: Sendable {
    let code: String
    let secret: String
}

enum SourceConnect {
    static func probe(_ address: URL, session: URLSession = SourceHTTP.session) async throws -> SourceServerInfo {
        var request = try JellyfinClient.request(SourceHTTP.url(address, "/System/Info/Public"), token: nil)
        request.timeoutInterval = 4
        let data: Data
        do { (data, _) = try await BoundedHTTPResponse.receive(request, session: session, maximum: 64 * 1024, expected: [200]) }
        catch is CancellationError { throw CancellationError() }
        catch { throw ClientError.unavailable }
        guard let value = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let serverID = SourceHTTP.identifier(value["Id"]), value["Version"] is String else { throw ClientError.discovery }
        let product = SourceHTTP.text(value["ProductName"], max: 64)
        let kind: SourceKind = product.localizedCaseInsensitiveContains("jellyfin") ? .jellyfin : .emby
        let name = SourceHTTP.text(value["ServerName"], max: 120)
        let generated = name.isEmpty || name.range(of: "^[0-9a-f]{12}$", options: .regularExpression) != nil
        return SourceServerInfo(kind: kind, serverID: serverID, name: generated ? kind.title : name,
                                version: SourceHTTP.text(value["Version"], max: 32), address: address)
    }

    static func signIn(_ server: SourceServerInfo, user: String, password: String) async throws -> SourceAccount {
        let name = user.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !name.isEmpty, name.utf8.count <= 256, password.utf8.count <= 1024 else { throw ClientError.invalidInput("Enter your user name.") }
        let request = try JellyfinClient.request(SourceHTTP.url(server.address, "/Users/AuthenticateByName"), token: nil, method: "POST",
                                                 body: ["Username": name, "Pw": password])
        do { return try await account(server, from: SourceHTTP.json(request, maximum: 1024 * 1024)) }
        catch ClientError.http(let status) where status == 401 || status == 403 {
            throw ClientError.invalidInput("The user name or password is not correct.")
        }
    }

    static func quickConnectEnabled(_ server: SourceServerInfo) async -> Bool {
        guard server.kind == .jellyfin, let request = try? JellyfinClient.request(SourceHTTP.url(server.address, "/QuickConnect/Enabled"), token: nil),
              let data = try? await SourceHTTP.data(request, maximum: 1024, expected: [200]) else { return false }
        return String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) == "true"
    }

    static func startQuickConnect(_ server: SourceServerInfo) async throws -> SourceQuickConnect {
        let request = try JellyfinClient.request(SourceHTTP.url(server.address, "/QuickConnect/Initiate"), token: nil, method: "POST")
        let value = try await SourceHTTP.json(request, maximum: 64 * 1024)
        guard let code = value["Code"] as? String, code.range(of: "^[0-9]{4,12}$", options: .regularExpression) != nil,
              let secret = value["Secret"] as? String, secret.range(of: "^[A-Za-z0-9]{16,256}$", options: .regularExpression) != nil
        else { throw ClientError.invalidResponse }
        return SourceQuickConnect(code: code, secret: secret)
    }

    static func awaitQuickConnect(_ server: SourceServerInfo, _ pending: SourceQuickConnect, until deadline: Date) async throws -> SourceAccount {
        while Date() < deadline {
            try await Task.sleep(for: .seconds(2))
            let state = try JellyfinClient.request(SourceHTTP.url(server.address, "/QuickConnect/Connect", ["Secret": pending.secret]), token: nil)
            let value = try await SourceHTTP.json(state, maximum: 64 * 1024)
            guard value["Authenticated"] as? Bool == true else { continue }
            let request = try JellyfinClient.request(SourceHTTP.url(server.address, "/Users/AuthenticateWithQuickConnect"), token: nil,
                                                     method: "POST", body: ["Secret": pending.secret])
            return try await account(server, from: SourceHTTP.json(request, maximum: 1024 * 1024))
        }
        throw ClientError.invalidInput("The code expired. Try again.")
    }

    private static func account(_ server: SourceServerInfo, from value: [String: Any]) throws -> SourceAccount {
        guard let token = value["AccessToken"] as? String, token.range(of: "^[A-Za-z0-9._-]{16,512}$", options: .regularExpression) != nil,
              let user = value["User"] as? [String: Any], let userID = SourceHTTP.identifier(user["Id"]) else { throw ClientError.invalidResponse }
        let serverID = SourceHTTP.identifier(value["ServerId"]) ?? server.serverID
        let account = SourceAccount(kind: server.kind, serverID: serverID, userID: userID, name: server.name,
                                    userName: SourceHTTP.text(user["Name"], max: 120), addresses: [server.address], token: token)
        deviceLibraryLog.info("operation=source-sign-in outcome=succeeded kind=\(server.kind.rawValue, privacy: .public)")
        return try account.validated()
    }
}
