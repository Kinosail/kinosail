import Foundation
import Security

@MainActor
final class DeviceLibraryHost {
    static let shared = DeviceLibraryHost()
    private static let portKey = "kinosail.device-library.port"
    private let accounts = SourceAccountStore()
    private let index = DeviceIndexStore()
    private let state = DeviceLocalState()
    private let streams = DeviceStreamRegistry()
    private let catalog: DeviceCatalog
    private var server: DeviceHTTPServer?
    private var configured = false
    private(set) var sources: [SourceAccount] = []

    private init() {
        catalog = DeviceCatalog(index: index)
    }

    static func isDeviceLibrary(_ viewer: Viewer?) -> Bool {
        viewer?.serverID == DeviceLibraryRouter.serverID
    }

    func start() async throws -> (server: ServerAddress, token: String) {
        let token = try DeviceLibraryToken.current()
        if !configured {
            sources = try await accounts.load()
            await catalog.configure(sources)
            configured = true
        }
        if let server, server.running { return (try Self.address(server.port), token) }
        server?.stop()
        let next = DeviceHTTPServer(token: token, handler: DeviceLibraryRouter(catalog: catalog, state: state, streams: streams))
        let preferred = UInt16(clamping: max(0, UserDefaults.standard.integer(forKey: Self.portKey)))
        let port = try await next.start(preferredPort: preferred)
        server = next
        UserDefaults.standard.set(Int(port), forKey: Self.portKey)
        deviceLibraryLog.info("operation=device-library-start outcome=ready sources=\(self.sources.count, privacy: .public)")
        return (try Self.address(port), token)
    }

    func prepare(_ saved: SavedSession) async throws -> SavedSession {
        guard Self.isDeviceLibrary(saved.viewer) else { return saved }
        let (server, token) = try await start()
        guard server != saved.server || token != saved.token else { return saved }
        let updated = try SavedSession(server: server, token: token, viewer: saved.viewer)
        try await SessionKeychain().save(updated)
        return updated
    }

    func add(_ account: SourceAccount) async throws {
        let account = try account.validated()
        var list = try await accounts.load()
        list.removeAll { $0.id == account.id }
        list.append(account)
        try await accounts.save(list)
        sources = list
        configured = true
        await catalog.configure(list)
    }

    func remove(_ id: String) async throws {
        var list = try await accounts.load()
        list.removeAll { $0.id == id }
        try await accounts.save(list)
        sources = list
        await catalog.configure(list)
    }

    func refresh() async {
        await catalog.invalidate()
    }

    func reset() async {
        try? await accounts.save([])
        sources = []
        await catalog.configure([])
        await index.clear()
        await state.clear()
        server?.stop()
        server = nil
        DeviceLibraryToken.discard()
        deviceLibraryLog.info("operation=device-library-reset outcome=cleared")
    }

    private static func address(_ port: UInt16) throws -> ServerAddress {
        try ServerAddress("http://127.0.0.1:\(port)")
    }
}

enum DeviceLibraryToken {
    private static var query: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "com.kinosail.player.swift.device-library.v1",
         kSecAttrAccount as String: "token", kSecAttrSynchronizable as String: false]
    }

    static func current() throws -> String {
        var lookup = query
        lookup[kSecReturnData as String] = true
        lookup[kSecMatchLimit as String] = kSecMatchLimitOne
        var result: CFTypeRef?
        let status = SecItemCopyMatching(lookup as CFDictionary, &result)
        if status == errSecSuccess, let data = result as? Data, let token = String(data: data, encoding: .utf8),
           token.range(of: "^[a-f0-9]{64}$", options: .regularExpression) != nil { return token }
        guard status == errSecSuccess || status == errSecItemNotFound else { throw ClientError.secureStorage }
        var bytes = [UInt8](repeating: 0, count: 32)
        guard SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes) == errSecSuccess else { throw ClientError.secureStorage }
        let token = bytes.map { String(format: "%02x", $0) }.joined()
        SecItemDelete(query as CFDictionary)
        var item = query
        item[kSecValueData as String] = Data(token.utf8)
        item[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        guard SecItemAdd(item as CFDictionary, nil) == errSecSuccess else { throw ClientError.secureStorage }
        return token
    }

    static func discard() {
        SecItemDelete(query as CFDictionary)
    }
}
