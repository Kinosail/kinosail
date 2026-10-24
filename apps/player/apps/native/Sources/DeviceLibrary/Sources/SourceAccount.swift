import CryptoKit
import Foundation
import Security

enum SourceKind: String, Codable, Sendable, CaseIterable {
    case jellyfin, emby, plex

    var title: String {
        switch self {
        case .jellyfin: "Jellyfin"
        case .emby: "Emby"
        case .plex: "Plex"
        }
    }
}

struct SourceAccount: Codable, Sendable, Identifiable, Hashable {
    let kind: SourceKind
    let serverID: String
    let userID: String
    var name: String
    var userName: String
    var addresses: [URL]
    var token: String

    var id: String { Self.identifier(kind: kind, serverID: serverID, userID: userID) }
    var address: URL { addresses[0] }

    static func identifier(kind: SourceKind, serverID: String, userID: String) -> String {
        let digest = SHA256.hash(data: Data("\(kind.rawValue)\n\(serverID)\n\(userID)".utf8))
        return digest.prefix(8).map { String(format: "%02x", $0) }.joined()
    }

    func validated() throws -> SourceAccount {
        guard !serverID.isEmpty, serverID.utf8.count <= 128, !userID.isEmpty, userID.utf8.count <= 128,
              name.utf8.count <= 256, userName.utf8.count <= 256, !token.isEmpty, token.utf8.count <= 4096,
              !addresses.isEmpty, addresses.count <= 16 else { throw ClientError.invalidResponse }
        for address in addresses { _ = try SourceAddress.validated(address) }
        return self
    }
}

enum SourceAddress {
    static func parse(_ input: String, defaultPort: Int) throws -> URL {
        var raw = input.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !raw.isEmpty, raw.utf8.count <= 2048 else { throw ClientError.invalidInput("Enter the server address.") }
        if !raw.contains("://") { raw = "http://" + raw }
        guard var parts = URLComponents(string: raw), let host = parts.host, !host.isEmpty else {
            throw ClientError.invalidInput("Enter a valid server address.")
        }
        if parts.port == nil, parts.scheme?.lowercased() == "http", !raw.dropFirst(7).contains(":") { parts.port = defaultPort }
        while parts.path.hasSuffix("/") { parts.path.removeLast() }
        parts.query = nil; parts.fragment = nil
        guard let url = parts.url else { throw ClientError.invalidInput("Enter a valid server address.") }
        return try validated(url)
    }

    static func validated(_ url: URL) throws -> URL {
        guard let parts = URLComponents(url: url, resolvingAgainstBaseURL: false),
              let scheme = parts.scheme?.lowercased(), let host = parts.host?.lowercased(), !host.isEmpty,
              parts.user == nil, parts.password == nil, parts.query == nil, parts.fragment == nil,
              scheme == "https" || scheme == "http" && ServerAddress.localHost(host),
              parts.port.map({ (1...65535).contains($0) }) ?? true, parts.path.utf8.count <= 256 else {
            throw ClientError.invalidInput("Use HTTPS for servers outside your home network.")
        }
        return url
    }
}

actor SourceAccountStore {
    private let service: String
    init(service: String = "com.kinosail.player.swift.sources.v1") { self.service = service }

    private var query: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service,
         kSecAttrAccount as String: "accounts", kSecAttrSynchronizable as String: false]
    }

    func load() throws -> [SourceAccount] {
        var query = query
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound { return [] }
        guard status == errSecSuccess, let data = result as? Data, data.count <= 512 * 1024 else { throw ClientError.secureStorage }
        let accounts = try JSONDecoder().decode([SourceAccount].self, from: data)
        guard accounts.count <= 32 else { throw ClientError.secureStorage }
        return try accounts.map { try $0.validated() }
    }

    func save(_ accounts: [SourceAccount]) throws {
        guard accounts.count <= 32 else { throw ClientError.invalidInput("Remove a library before adding another.") }
        let data = try JSONEncoder().encode(try accounts.map { try $0.validated() })
        if accounts.isEmpty {
            let status = SecItemDelete(query as CFDictionary)
            guard status == errSecSuccess || status == errSecItemNotFound else { throw ClientError.secureStorage }
            return
        }
        let values: [String: Any] = [kSecValueData as String: data,
                                    kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly]
        let status = SecItemUpdate(query as CFDictionary, values as CFDictionary)
        if status == errSecItemNotFound {
            guard SecItemAdd(query.merging(values) { _, new in new } as CFDictionary, nil) == errSecSuccess else { throw ClientError.secureStorage }
        } else if status != errSecSuccess { throw ClientError.secureStorage }
    }
}
