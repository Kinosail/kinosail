#if os(tvOS) && targetEnvironment(simulator)
import Foundation
import Testing
@testable import KinosailPlayer

// Opt-in setup for the disposable loopback navigation fixture. This file is
// test-only and must never pair a production profile or run on another device.
@Suite(.enabled(if: ProcessInfo.processInfo.environment["KINOSAIL_TV_QUALITY_SEED"] == "1"))
@MainActor struct TVQualitySessionSeed {
    @Test func seedDisposableSimulatorSession() async throws {
        let environment = ProcessInfo.processInfo.environment
        let mode = environment["KINOSAIL_TV_QUALITY_FIXTURE_MODE"] ?? "loaded"
        let identity = environment["KINOSAIL_TV_QUALITY_FIXTURE_ID"] ?? "tv-polish-qa"
        try #require(["loaded", "empty", "failed", "pending"].contains(mode))
        try #require(identity.range(of: "^tv-polish-qa(?:-[a-z0-9-]{1,64})?$", options: .regularExpression) != nil)
        let state = mode == "pending" ? "loaded" : mode
        let delay = mode == "pending" ? 15 : 0
        let stateURL = URL(string: "http://127.0.0.1:38359/qa/state?mode=\(state)&delay=\(delay)&identity=\(identity)")!
        let (_, response) = try await URLSession.shared.data(from: stateURL)
        try #require((response as? HTTPURLResponse)?.statusCode == 200)
        let server = try ServerAddress("http://127.0.0.1:38359")
        let client = try ServerClient(server: server, token: "tv-polish-fixture-token")
        let viewer = try await client.viewer()
        try #require(viewer.id == "qa")
        try #require(viewer.serverID == identity)
        let keychain = SessionKeychain()
        if let existing = try await keychain.restore() {
            guard existing.server == server,
                  existing.viewer.id == "qa",
                  existing.token == "tv-polish-fixture-token" else {
                throw ClientError.invalidInput("QA setup requires a disposable simulator without another saved session.")
            }
        }
        try await keychain.save(SavedSession(server: server, token: "tv-polish-fixture-token", viewer: viewer))
        await client.close()
    }
}
#endif
