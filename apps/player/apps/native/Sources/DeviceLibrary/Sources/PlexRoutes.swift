import Foundation

final class PlexRoutes: @unchecked Sendable {
    static let shared = PlexRoutes()
    private let lock = NSLock()
    private var chosen: [String: URL] = [:]

    func current(_ account: SourceAccount) -> URL {
        lock.withLock { chosen[account.id] ?? account.address }
    }

    private func remember(_ address: URL, for account: SourceAccount) {
        lock.withLock { chosen[account.id] = address }
    }

    func resolve(_ account: SourceAccount) async throws -> URL {
        let previous = current(account)
        for address in [previous] + account.addresses.filter({ $0 != previous }).prefix(15) {
            if await Self.reachable(address, server: account.serverID, token: account.token) {
                remember(address, for: account)
                if address != previous { deviceLibraryLog.info("operation=plex-route outcome=switched local=\(ServerAddress.localHost(address.host ?? ""), privacy: .public)") }
                return address
            }
        }
        deviceLibraryLog.warning("operation=plex-route outcome=unreachable connections=\(account.addresses.count, privacy: .public)")
        throw ClientError.unavailable
    }

    static func reachable(_ address: URL, server: String, token: String) async -> Bool {
        guard var request = try? URLRequest(url: SourceHTTP.url(address, "/identity")) else { return false }
        PlexAccounts.sign(&request, token: token)
        request.timeoutInterval = 4
        guard let data = try? await SourceHTTP.data(request, maximum: 64 * 1024, expected: [200]),
              let value = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let container = value["MediaContainer"] as? [String: Any] else { return false }
        return SourceHTTP.identifier(container["machineIdentifier"]) == server
    }

    static func probe(_ address: URL, session: URLSession = SourceHTTP.session) async throws -> SourceServerInfo {
        var request = URLRequest(url: try SourceHTTP.url(address, "/identity"))
        PlexAccounts.sign(&request, token: nil)
        request.timeoutInterval = 4
        let data: Data
        do { (data, _) = try await BoundedHTTPResponse.receive(request, session: session, maximum: 64 * 1024, expected: [200]) }
        catch is CancellationError { throw CancellationError() }
        catch { throw ClientError.unavailable }
        guard let value = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let container = value["MediaContainer"] as? [String: Any],
              let serverID = SourceHTTP.identifier(container["machineIdentifier"]) else { throw ClientError.discovery }
        return SourceServerInfo(kind: .plex, serverID: serverID, name: "Plex Media Server", version: SourceHTTP.text(container["version"], max: 32), address: address)
    }
}
