import Foundation
import Security

actor SourceSharedStore {
    static let shared = SourceSharedStore()
    private let service: String
    init(service: String = "com.kinosail.player.swift.sources.shared.v1") { self.service = service }

    private func query(_ id: String? = nil) -> [String: Any] {
        var query: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service,
                                    kSecAttrSynchronizable as String: true]
        if let id { query[kSecAttrAccount as String] = id }
        return query
    }

    func load() -> [SourceAccount] {
        var query = query()
        query[kSecReturnData as String] = true
        query[kSecReturnAttributes as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitAll
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        guard status == errSecSuccess, let items = result as? [[String: Any]] else {
            if status != errSecItemNotFound { deviceLibraryLog.error("operation=source-share-load outcome=failed status=\(status, privacy: .public)") }
            return []
        }
        return items.prefix(32).compactMap { item in
            guard let data = item[kSecValueData as String] as? Data, data.count <= 64 * 1024,
                  let account = try? JSONDecoder().decode(SourceAccount.self, from: data).validated(),
                  item[kSecAttrAccount as String] as? String == account.id else { return nil }
            return account
        }
    }

    func publish(_ account: SourceAccount) {
        guard let data = try? JSONEncoder().encode(account.validated()) else { return }
        let values: [String: Any] = [kSecValueData as String: data, kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlock,
                                     kSecAttrLabel as String: "Kinosail Player · \(account.kind.title)"]
        var status = SecItemUpdate(query(account.id) as CFDictionary, values as CFDictionary)
        if status == errSecItemNotFound { status = SecItemAdd(query(account.id).merging(values) { _, new in new } as CFDictionary, nil) }
        deviceLibraryLog.log(level: status == errSecSuccess ? .info : .error,
                             "operation=source-share-publish outcome=\(status == errSecSuccess ? "stored" : "failed", privacy: .public) status=\(status, privacy: .public)")
    }

    func withdraw(_ id: String) {
        let status = SecItemDelete(query(id) as CFDictionary)
        if status != errSecSuccess && status != errSecItemNotFound {
            deviceLibraryLog.error("operation=source-share-withdraw outcome=failed status=\(status, privacy: .public)")
        }
    }
}

extension SourceConnect {
    static func adopt(_ shared: SourceAccount) async throws -> SourceAccount {
        var failure: (any Error)?
        for address in shared.addresses.prefix(4) {
            do {
                let server = try await probe(address)
                guard server.kind == shared.kind, server.serverID == shared.serverID else { throw ClientError.discovery }
                let named = SourceServerInfo(kind: server.kind, serverID: server.serverID, name: shared.name, version: server.version, address: address)
                if server.kind == .jellyfin, await quickConnectEnabled(server), let own = try? await approveSelf(named, with: shared) {
                    return own
                }
                let check = try JellyfinClient.request(SourceHTTP.url(address, "/Users/\(shared.userID)"), token: shared.token)
                _ = try await SourceHTTP.data(check, maximum: 1024 * 1024, expected: [200])
                var account = shared
                account.addresses = [address] + shared.addresses.filter { $0 != address }
                deviceLibraryLog.info("operation=source-adopt outcome=shared-token kind=\(server.kind.rawValue, privacy: .public)")
                return try account.validated()
            } catch ClientError.http(let status) where status == 401 || status == 403 {
                throw ClientError.invalidInput("This sign-in has expired. Sign in to \(shared.name) again on this device.")
            } catch {
                failure = error
            }
        }
        deviceLibraryLog.warning("operation=source-adopt outcome=unreachable kind=\(shared.kind.rawValue, privacy: .public)")
        throw failure is CancellationError ? CancellationError() : ClientError.unavailable
    }

    static func approve(code: String, account: SourceAccount) async throws {
        guard account.kind == .jellyfin, code.range(of: "^[0-9]{4,12}$", options: .regularExpression) != nil else {
            throw ClientError.invalidInput("This code is not valid.")
        }
        let request = try JellyfinClient.request(SourceHTTP.url(account.address, "/QuickConnect/Authorize", ["code": code]),
                                                 token: account.token, method: "POST")
        do { _ = try await SourceHTTP.data(request, maximum: 64 * 1024, expected: [200, 204]) }
        catch ClientError.http(let status) where status == 404 || status == 400 {
            throw ClientError.invalidInput("This code expired. Show a new code on your TV and try again.")
        }
        deviceLibraryLog.info("operation=quick-connect-approve outcome=approved")
    }

    private static func approveSelf(_ server: SourceServerInfo, with shared: SourceAccount) async throws -> SourceAccount {
        let pending = try await startQuickConnect(server)
        var signed = shared
        signed.addresses = [server.address]
        try await approve(code: pending.code, account: signed)
        let account = try await awaitQuickConnect(server, pending, until: Date().addingTimeInterval(20), interval: .milliseconds(300))
        deviceLibraryLog.info("operation=source-adopt outcome=own-token kind=jellyfin")
        return account
    }
}
