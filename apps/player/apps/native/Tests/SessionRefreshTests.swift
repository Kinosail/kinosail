import Foundation
import Testing
@testable import KinosailPlayer

struct SessionRefreshTests {
    @Test func backgroundRefreshCannotReplaceNewPairingOrRestoreSignedOutCredentials() async throws {
        let keychain = SessionKeychain(service: "com.kinosail.tests.refresh.\(UUID().uuidString)")
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true), "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let old = try SavedSession(server: ServerAddress("https://old.example"), token: "old-token", viewer: viewer)
        let paired = try SavedSession(server: ServerAddress("https://new.example"), token: "new-token", viewer: viewer)
        do {
            try await keychain.save(old)
            #expect(try await keychain.refresh(old))
            try await keychain.save(paired)
            #expect(try await keychain.refresh(old) == false)
            #expect(try await keychain.restore()?.token == paired.token)
            try await keychain.clear()
            #expect(try await keychain.refresh(old) == false)
            #expect(try await keychain.restore() == nil)
        } catch {
            try? await keychain.clear()
            throw error
        }
    }
}
