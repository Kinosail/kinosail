import Foundation

struct PlexPin: Sendable, Hashable {
    let id: Int
    let code: String
}

struct PlexServer: Identifiable, Hashable, Sendable {
    let id: String
    let name: String
    let owned: Bool
    let token: String
    let connections: [URL]
}

enum PlexAccounts {
    static let base: URL = {
        #if DEBUG
        if let raw = ProcessInfo.processInfo.environment["KINOSAIL_PLEX_TV"], let url = URL(string: raw),
           url.scheme == "http", url.host == "127.0.0.1", url.port != nil { return url }
        #endif
        return URL(string: "https://plex.tv")!
    }()

    private static let platformVersion: String = {
        let version = ProcessInfo.processInfo.operatingSystemVersion
        return "\(version.majorVersion).\(version.minorVersion)"
    }()

    static func sign(_ request: inout URLRequest, token: String?) {
        request.httpShouldHandleCookies = false
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.setValue("Kinosail Player", forHTTPHeaderField: "X-Plex-Product")
        request.setValue(SourceHTTP.version, forHTTPHeaderField: "X-Plex-Version")
        request.setValue(SourceHTTP.installID, forHTTPHeaderField: "X-Plex-Client-Identifier")
        #if os(tvOS)
        request.setValue("tvOS", forHTTPHeaderField: "X-Plex-Platform")
        #else
        request.setValue("iOS", forHTTPHeaderField: "X-Plex-Platform")
        #endif
        request.setValue(platformVersion, forHTTPHeaderField: "X-Plex-Platform-Version")
        request.setValue(SourceHTTP.deviceName, forHTTPHeaderField: "X-Plex-Device")
        request.setValue(SourceHTTP.deviceName, forHTTPHeaderField: "X-Plex-Device-Name")
        if let token { request.setValue(token, forHTTPHeaderField: "X-Plex-Token") }
    }

    static func request(_ path: String, _ query: [String: String] = [:], token: String?, method: String = "GET") throws -> URLRequest {
        var request = URLRequest(url: try SourceHTTP.url(base, path, query))
        request.httpMethod = method
        sign(&request, token: token)
        return request
    }

    static func startPin() async throws -> PlexPin {
        let data = try await SourceHTTP.data(request("/api/v2/pins", ["strong": "false"], token: nil, method: "POST"), maximum: 64 * 1024, expected: [200, 201])
        guard let value = try JSONSerialization.jsonObject(with: data) as? [String: Any], let id = value["id"] as? Int, id > 0, let code = value["code"] as? String,
              code.range(of: "^[A-Z0-9]{4,8}$", options: .regularExpression) != nil else { throw ClientError.invalidResponse }
        return PlexPin(id: id, code: code)
    }

    static func authURL(_ pin: PlexPin, forward: Bool) -> URL? {
        var parts = URLComponents(url: base, resolvingAgainstBaseURL: false)
        if base.host == "plex.tv" { parts?.host = "app.plex.tv" }
        parts?.path = "/auth"
        var fragment = URLComponents()
        fragment.queryItems = [URLQueryItem(name: "clientID", value: SourceHTTP.installID), URLQueryItem(name: "code", value: pin.code),
                               URLQueryItem(name: "context[device][product]", value: "Kinosail Player")]
        if forward { fragment.queryItems?.append(URLQueryItem(name: "forwardUrl", value: "kinosail://plex-auth")) }
        parts?.percentEncodedFragment = "?" + (fragment.percentEncodedQuery ?? "")
        return parts?.url
    }

    static func linkPage() -> String {
        base.host == "plex.tv" ? "plex.tv/link" : (base.host ?? "") + ":\(base.port ?? 80)/link"
    }

    static func awaitToken(_ pin: PlexPin, until deadline: Date, interval: Duration = .seconds(1)) async throws -> String {
        while Date() < deadline {
            try await Task.sleep(for: interval)
            let value: [String: Any]
            do { value = try await SourceHTTP.json(request("/api/v2/pins/\(pin.id)", token: nil), maximum: 64 * 1024) }
            catch ClientError.http(404) { break }
            if let token = value["authToken"] as? String, valid(token) { return token }
        }
        throw ClientError.invalidInput("The code expired. Try again.")
    }

    static func user(token: String) async throws -> (id: String, name: String) {
        let value: [String: Any]
        do { value = try await SourceHTTP.json(request("/api/v2/user", token: token), maximum: 512 * 1024) }
        catch ClientError.http(401) { throw ClientError.invalidInput("Plex did not accept this sign-in. Try again.") }
        guard let id = SourceHTTP.identifier(value["id"]) else { throw ClientError.invalidResponse }
        let name = SourceHTTP.text(value["title"], max: 120)
        return (id, name.isEmpty ? SourceHTTP.text(value["username"], max: 120) : name)
    }

    static func servers(token: String) async throws -> [PlexServer] {
        let data = try await SourceHTTP.data(request("/api/v2/resources", ["includeHttps": "1", "includeRelay": "1", "includeIPv6": "0"], token: token),
                                             maximum: 4 * 1024 * 1024, expected: [200])
        guard let rows = try JSONSerialization.jsonObject(with: data) as? [[String: Any]] else { throw ClientError.invalidResponse }
        return rows.prefix(64).compactMap { row in
            guard (row["provides"] as? String ?? "").split(separator: ",").contains("server"),
                  let id = SourceHTTP.identifier(row["clientIdentifier"]) else { return nil }
            let access = row["accessToken"] as? String ?? token
            guard valid(access) else { return nil }
            let ranked = (row["connections"] as? [[String: Any]] ?? []).prefix(16).compactMap { connection -> (URL, Int)? in
                guard let raw = connection["uri"] as? String, let url = URL(string: raw), let checked = try? SourceAddress.validated(url) else { return nil }
                let relay = connection["relay"] as? Bool ?? false, local = connection["local"] as? Bool ?? false
                return (checked, relay ? 2 : local ? 0 : 1)
            }.sorted { $0.1 < $1.1 }.map(\.0)
            guard !ranked.isEmpty else { return nil }
            let name = SourceHTTP.text(row["name"], max: 120)
            return PlexServer(id: id, name: name.isEmpty ? "Plex" : name, owned: row["owned"] as? Bool ?? false, token: access, connections: ranked)
        }
        .sorted { ($0.owned ? 0 : 1, $0.name) < ($1.owned ? 0 : 1, $1.name) }
    }

    static func account(_ server: PlexServer, user: (id: String, name: String)) async throws -> SourceAccount {
        let reachable = await withTaskGroup(of: (Int, URL)?.self) { group in
            for (index, address) in server.connections.enumerated() {
                group.addTask { await PlexRoutes.reachable(address, server: server.id, token: server.token) ? (index, address) : nil }
            }
            var found: [(Int, URL)] = []
            for await result in group { if let result { found.append(result) } }
            return found.sorted { $0.0 < $1.0 }.map(\.1)
        }
        guard let first = reachable.first else {
            deviceLibraryLog.warning("operation=plex-connect outcome=unreachable connections=\(server.connections.count, privacy: .public)")
            throw ClientError.invalidInput("Kinosail could not reach \(server.name). Check that the server is on and try again.")
        }
        let addresses = [first] + server.connections.filter { $0 != first }
        let account = SourceAccount(kind: .plex, serverID: server.id, userID: user.id, name: server.name, userName: user.name,
                                    addresses: addresses, token: server.token)
        deviceLibraryLog.info("operation=source-sign-in outcome=succeeded kind=plex")
        return try account.validated()
    }

    static func valid(_ token: String) -> Bool {
        token.range(of: "^[A-Za-z0-9._~-]{8,512}$", options: .regularExpression) != nil
    }
}
