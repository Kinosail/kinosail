import Foundation
import Security

struct SavedSession: Sendable {
    let server: ServerAddress
    let token: String
    let viewer: Viewer

    init(server: ServerAddress, token: String, viewer: Viewer) throws {
        self.server = try ServerAddress(server.url.absoluteString)
        self.token = try Input.secret(token)
        self.viewer = try Viewer(viewer.json)
    }

    init(data: Data) throws {
        let value = try StrictJSON.decode(data, maximum: 16_384).object(allowing: ["version", "server", "token", "viewer"])
        guard try value.requiredNumber("version", integer: true) == 1 else { throw ClientError.invalidResponse }
        try self.init(server: ServerAddress(value.text("server", required: true)),
                      token: value.text("token", max: 2048, required: true), viewer: Viewer(value.required("viewer")))
    }

    func encoded() throws -> Data {
        let raw = JSONValue.object(["version": .number(1), "server": .string(server.url.absoluteString),
                                    "token": .string(token), "viewer": viewer.json])
        let data = try JSONEncoder().encode(raw)
        _ = try SavedSession(data: data)
        return data
    }
}

actor SessionKeychain {
    private let service: String
    init(service: String = "com.kinosail.player.swift.session.v1") { self.service = service }
    private var query: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service,
         kSecAttrAccount as String: "active", kSecAttrSynchronizable as String: false]
    }

    func restore() throws -> SavedSession? {
        var query = query
        query[kSecReturnData as String] = true
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess, let data = result as? Data else { throw ClientError.secureStorage }
        return try SavedSession(data: data)
    }

    func save(_ session: SavedSession) throws {
        let data = try session.encoded()
        let values: [String: Any] = [kSecValueData as String: data,
                                    kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly]
        let status = SecItemUpdate(query as CFDictionary, values as CFDictionary)
        if status == errSecItemNotFound {
            guard SecItemAdd(query.merging(values) { _, new in new } as CFDictionary, nil) == errSecSuccess else {
                throw ClientError.secureStorage
            }
        } else if status != errSecSuccess { throw ClientError.secureStorage }
    }

    func clear() throws {
        let status = SecItemDelete(query as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else { throw ClientError.secureStorage }
    }

    /// Background identity refresh must never replace a newer pairing's token.
    func refresh(_ session: SavedSession) throws -> Bool {
        guard let current = try restore(), current.server == session.server, current.token == session.token else { return false }
        try save(session)
        return true
    }

    func discard(_ candidate: SavedSession) throws {
        if let current = try restore(), current.server == candidate.server, current.token == candidate.token { try clear() }
    }
}
