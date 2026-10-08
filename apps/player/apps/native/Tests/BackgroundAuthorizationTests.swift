import Foundation
import Testing
@testable import KinosailPlayer

#if os(iOS)
extension SessionRefreshTests {
    @Test func coldBackgroundAuthorizationUsesMatchingSavedCredentials() async throws {
        try await withBackgroundAuthorizationProbe { probe in
            let saved = try backgroundSavedSession(probe.fixture, profile: "viewer", token: "cold-token")
            try await probe.keychain.save(saved)
            let revision = await probe.engine.authorizationRevision()
            let restored = try #require(await probe.keychain.restore())
            let access = try backgroundAccess(restored)
            try await probe.engine.authorize(access, ifCurrent: revision)
            #expect(try await probe.engine.snapshot(access.scope).isEmpty)
            let request = try await backgroundManifestRequest(probe, access: access)
            #expect(request.value(forHTTPHeaderField: "Authorization") == "Bearer cold-token")
            #expect(request.value(forHTTPHeaderField: "X-Kinosail-Viewer-Profile") == "viewer")
        }
    }

    @Test(arguments: [false, true])
    func lockInvalidatesPendingBackgroundAuthorizationEvenWhenAlreadyLocked(_ initiallyAuthorized: Bool) async throws {
        try await withBackgroundAuthorizationProbe { probe in
            let saved = try backgroundSavedSession(probe.fixture, profile: "viewer", token: "old-token")
            try await probe.keychain.save(saved)
            let access = try backgroundAccess(saved)
            if initiallyAuthorized { try await probe.engine.authorize(access) }
            let revision = await probe.engine.authorizationRevision()
            let restored = try #require(await probe.keychain.restore())
            try await probe.keychain.clear()
            await probe.engine.lock()
            try await probe.engine.authorize(backgroundAccess(restored), ifCurrent: revision)
            await #expect(throws: ClientError.http(403)) { try await probe.engine.snapshot(access.scope) }
            #expect(try await probe.keychain.restore() == nil)
            #expect(probe.fixture.requests.isEmpty)
        }
    }

    @Test(arguments: [false, true])
    func newerPairingSurvivesStaleBackgroundAuthorizationAndErrorLock(_ sameProfile: Bool) async throws {
        try await withBackgroundAuthorizationProbe { probe in
            let original = try backgroundSavedSession(probe.fixture, profile: "viewer", token: "old-token")
            let paired = try backgroundSavedSession(probe.fixture, profile: sameProfile ? "viewer" : "new-viewer", token: "new-token")
            try await probe.keychain.save(original)
            let oldAccess = try backgroundAccess(original)
            try await probe.engine.authorize(oldAccess)
            let staleRevision = await probe.engine.authorizationRevision()
            let restored = try #require(await probe.keychain.restore())
            try await probe.keychain.save(paired)
            let currentAccess = try backgroundAccess(paired)
            try await probe.engine.authorize(currentAccess)
            try await probe.engine.authorize(backgroundAccess(restored), ifCurrent: staleRevision)
            // Both missing-credential and restore-error fallbacks use this same
            // conditional operation. Neither may lock the newer pairing.
            await probe.engine.lock(ifCurrent: staleRevision)
            #expect(try await probe.engine.snapshot(currentAccess.scope).isEmpty)
            if !sameProfile {
                await #expect(throws: ClientError.http(403)) { try await probe.engine.snapshot(oldAccess.scope) }
            }
            let request = try await backgroundManifestRequest(probe, access: currentAccess)
            #expect(request.value(forHTTPHeaderField: "Authorization") == "Bearer new-token")
            #expect(request.value(forHTTPHeaderField: "X-Kinosail-Viewer-Profile") == paired.viewer.id)
            #expect(try await probe.keychain.restore()?.token == "new-token")
        }
    }
    @Test(arguments: [false, true])
    func matchingConditionalLockInvalidatesItsTicketEvenWhenAlreadyLocked(_ initiallyAuthorized: Bool) async throws {
        try await withBackgroundAuthorizationProbe { probe in
            let saved = try backgroundSavedSession(probe.fixture, profile: "viewer", token: "old-token")
            try await probe.keychain.save(saved)
            let access = try backgroundAccess(saved)
            if initiallyAuthorized { try await probe.engine.authorize(access) }
            let revision = await probe.engine.authorizationRevision()
            let restored = try #require(await probe.keychain.restore())
            try await probe.keychain.clear()
            await probe.engine.lock(ifCurrent: revision)
            try await probe.engine.authorize(backgroundAccess(restored), ifCurrent: revision)
            await #expect(throws: ClientError.http(403)) { try await probe.engine.snapshot(access.scope) }
            #expect(try await probe.keychain.restore() == nil)
            #expect(probe.fixture.requests.isEmpty)
        }
    }

    @Test func staleOperationsPreserveNewerPendingAuthorizationTicket() async throws {
        try await withBackgroundAuthorizationProbe { probe in
            let original = try backgroundSavedSession(probe.fixture, profile: "original", token: "old-token")
            let paired = try backgroundSavedSession(probe.fixture, profile: "paired", token: "new-token")
            let pending = try backgroundSavedSession(probe.fixture, profile: "pending", token: "pending-token")
            let oldAccess = try backgroundAccess(original)
            let pairedAccess = try backgroundAccess(paired)
            let pendingAccess = try backgroundAccess(pending)
            try await probe.engine.authorize(oldAccess)
            let staleRevision = await probe.engine.authorizationRevision()
            try await probe.keychain.save(paired)
            try await probe.engine.authorize(pairedAccess)
            let currentRevision = await probe.engine.authorizationRevision()
            try await probe.keychain.save(pending)
            let restored = try #require(await probe.keychain.restore())
            try await probe.engine.authorize(oldAccess, ifCurrent: staleRevision)
            await probe.engine.lock(ifCurrent: staleRevision)
            try await probe.engine.authorize(backgroundAccess(restored), ifCurrent: currentRevision)
            #expect(try await probe.engine.snapshot(pendingAccess.scope).isEmpty)
            await #expect(throws: ClientError.http(403)) { try await probe.engine.snapshot(pairedAccess.scope) }
            await #expect(throws: ClientError.http(403)) { try await probe.engine.snapshot(oldAccess.scope) }
            #expect(probe.fixture.requests.isEmpty)
        }
    }

    @Test(arguments: [false, true])
    func failedConditionalReauthorizationKeepsItsFallbackLockEffective(_ failsJournalWrite: Bool) async throws {
        try await withBackgroundAuthorizationProbe { probe in
            let old = try backgroundSavedSession(probe.fixture, profile: "viewer", token: "old-token")
            let replacement = try backgroundSavedSession(probe.fixture, profile: "viewer", token: "new-token")
            let access = try backgroundAccess(old)
            let id = "aaaaaaaaaaaaaaaa", key = String(repeating: "b", count: 64)
            let path = "/api/v1/downloads/" + id
            let hash = String(repeating: "c", count: 64)
            let manifest = JSONValue.object(["version": .number(1), "id": .string(id), "size": .number(1),
                "sha256": .string(hash), "chunkSize": .number(8 * 1024 * 1024), "chunks": .array([.string(hash)])])
            let manifestData = try JSONEncoder().encode(manifest)
            FixtureURLProtocol.entries.withLock {
                $0[probe.fixture.host]?.routes[path + "/manifest"] = .init(data: manifestData, status: 200, headers: [:])
                $0[probe.fixture.host]?.routes[path + "/file"] = .init(data: Data(), status: 206, headers: [:], hold: true)
            }
            try await probe.engine.authorize(access)
            try await probe.engine.enqueuePreparation(scope: access.scope, key: key,
                uri: "https://" + probe.fixture.host + path + "/file", kind: "audio", wifiOnly: false, quota: 0)
            var status: String?
            for _ in 0..<200 {
                status = try await probe.engine.snapshot(access.scope).first { $0.key == key }?.status
                if status == "downloading", probe.fixture.requests.contains(where: { $0.url?.path == path + "/file" }) { break }
                try await Task.sleep(for: .milliseconds(5))
            }
            try #require(status == "downloading")
            let initialFiles = probe.fixture.requests.filter { $0.url?.path == path + "/file" }
            try #require(initialFiles.count == 1)
            #expect(initialFiles[0].value(forHTTPHeaderField: "Authorization") == "Bearer old-token")
            let journal = probe.directory.appendingPathComponent(access.scope).appendingPathComponent(key + ".transfer")
            let journalValues = try journal.resourceValues(forKeys: [.isRegularFileKey])
            try #require(journalValues.isRegularFile == true)
            try await probe.keychain.save(replacement)
            let revision = await probe.engine.authorizationRevision()
            let saved = try #require(await probe.keychain.restore())
            if failsJournalWrite {
                // A nonempty directory cannot be replaced by an atomic journal write.
                try FileManager.default.removeItem(at: journal)
                try FileManager.default.createDirectory(at: journal, withIntermediateDirectories: false)
                try Data([0]).write(to: journal.appendingPathComponent("write-blocker"))
            }
            var threw = false
            do { try await probe.engine.authorize(backgroundAccess(saved), ifCurrent: revision) }
            catch {
                threw = true
                let failure = error as NSError
                #expect(failure.domain == NSCocoaErrorDomain || failure.domain == NSPOSIXErrorDomain)
                // Match the stock delegate's restore/authorize error fallback.
                await probe.engine.lock(ifCurrent: revision)
            }
            #expect(threw == failsJournalWrite)
            if failsJournalWrite {
                await #expect(throws: ClientError.http(403)) { try await probe.engine.snapshot(access.scope) }
                #expect(probe.fixture.requests.filter { $0.url?.path == path + "/file" }.count == 1)
            } else {
                for _ in 0..<200 where probe.fixture.requests.filter({ $0.url?.path == path + "/file" }).count < 2 {
                    try await Task.sleep(for: .milliseconds(5))
                }
                let files = probe.fixture.requests.filter { $0.url?.path == path + "/file" }
                try #require(files.count == 2)
                #expect(files[1].value(forHTTPHeaderField: "Authorization") == "Bearer new-token")
                #expect(files[1].value(forHTTPHeaderField: "X-Kinosail-Viewer-Profile") == "viewer")
                #expect(try await probe.engine.snapshot(access.scope).first { $0.key == key }?.status == "downloading")
                #expect(try journal.resourceValues(forKeys: [.isRegularFileKey]).isRegularFile == true)
            }
        }
    }
}

private struct BackgroundAuthorizationProbe: Sendable {
    let directory: URL
    let engine: VerifiedDownloads
    let fixture: HTTPFixture
    let keychain: SessionKeychain
}

private func withBackgroundAuthorizationProbe(_ operation: @Sendable (BackgroundAuthorizationProbe) async throws -> Void) async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at: directory) }
    let fixture = try HTTPFixture(body: "{}")
    defer { fixture.remove() }
    let configuration = URLSessionConfiguration.ephemeral
    configuration.protocolClasses = [FixtureURLProtocol.self]
    let engine = VerifiedDownloads(directory: directory, configuration: configuration)
    let keychain = SessionKeychain(service: "com.kinosail.tests.background-auth.\(UUID().uuidString)")
    do {
        try await operation(BackgroundAuthorizationProbe(directory: directory, engine: engine, fixture: fixture, keychain: keychain))
        try await keychain.clear()
        await engine.close()
    } catch {
        try? await keychain.clear()
        await engine.close()
        throw error
    }
}

private func backgroundSavedSession(_ fixture: HTTPFixture, profile: String, token: String) throws -> SavedSession {
    let viewer = try Viewer(.object(["server": .string("Fixture"), "serverId": .string("server"),
        "viewer": .object(["id": .string(profile), "name": .string("Viewer"), "owner": .bool(true),
            "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
    return try SavedSession(server: ServerAddress("https://" + fixture.host), token: token, viewer: viewer)
}

private func backgroundAccess(_ saved: SavedSession) throws -> DownloadAuthorization {
    try DownloadAuthorization(server: saved.server, serverID: saved.viewer.serverID, profileID: saved.viewer.id, token: saved.token)
}

private func backgroundManifestRequest(_ probe: BackgroundAuthorizationProbe, access: DownloadAuthorization) async throws -> URLRequest {
    let id = "aaaaaaaaaaaaaaaa"
    let path = "/api/v1/downloads/" + id
    FixtureURLProtocol.entries.withLock {
        $0[probe.fixture.host]?.routes[path + "/manifest"] = .init(data: Data(), status: 200, headers: [:], hold: true)
    }
    let began = ContinuousClock.now
    try await probe.engine.enqueuePreparation(scope: access.scope, key: String(repeating: "b", count: 64),
        uri: "https://" + probe.fixture.host + path + "/file", kind: "audio", wifiOnly: false, quota: 0)
    let enqueued = ContinuousClock.now
    var lastPoll = enqueued, longestGap = Duration.zero
    var polls = 0
    for _ in 0..<200 where probe.fixture.requests.isEmpty {
        try await Task.sleep(for: .milliseconds(5))
        let now = ContinuousClock.now
        longestGap = max(longestGap, lastPoll.duration(to: now))
        lastPoll = now
        polls += 1
    }
    // Freeze the original readiness decision before emitting the timing receipt.
    let observed = probe.fixture.requests
    let readinessFinished = ContinuousClock.now
    // Observe late delivery without changing the original readiness assertion.
    let diagnosticDeadline = readinessFinished.advanced(by: .seconds(10))
    var diagnosticInterrupted = false
    if observed.isEmpty {
        while probe.fixture.requests.isEmpty && ContinuousClock.now < diagnosticDeadline {
            do { try await Task.sleep(for: .milliseconds(5)) }
            catch { diagnosticInterrupted = true; break }
        }
    }
    let diagnosticFinished = ContinuousClock.now
    let milliseconds: (Duration) -> Double = { duration in
        let parts = duration.components
        return Double(parts.seconds) * 1000 + Double(parts.attoseconds) / 1e15
    }
    let delivery = FixtureURLProtocol.entries.withLock { $0[probe.fixture.host] }
    let diagnosticRequest = delivery?.requests.first
    let diagnosticRequestMatches = delivery?.requests.count == 1
        && diagnosticRequest?.url?.path == path + "/manifest"
        && diagnosticRequest?.value(forHTTPHeaderField: "Authorization") == access.header
        && diagnosticRequest?.value(forHTTPHeaderField: "X-Kinosail-Viewer-Profile") == access.profileID
    let diagnostic: [String: Any] = ["enqueueFinishedMs": milliseconds(began.duration(to: enqueued)),
        "readinessFinishedMs": milliseconds(began.duration(to: readinessFinished)),
        "polls": polls, "iterationBound": 200, "longestPollGapMs": milliseconds(longestGap),
        "callerTaskPriority": Task.currentPriority.rawValue, "initialRequestCount": observed.count,
        "diagnosticDeadlineMs": 10_000, "diagnosticFinishedMs": milliseconds(began.duration(to: diagnosticFinished)),
        "diagnosticInterrupted": diagnosticInterrupted,
        "diagnosticRequestCount": delivery?.requests.count ?? 0,
        "diagnosticRequestMatches": diagnosticRequestMatches,
        "admissionElapsedMs": (delivery?.requestAdmissions ?? []).map { milliseconds(began.duration(to: $0)) },
        "arrivalElapsedMs": (delivery?.requestArrivals ?? []).map { milliseconds(began.duration(to: $0)) }]
    if let data = try? JSONSerialization.data(withJSONObject: diagnostic, options: [.sortedKeys]),
       let text = String(data: data, encoding: .utf8) { print("BACKGROUND_MANIFEST_TIMING \(text)") }
    let request = try #require(observed.first)
    #expect(probe.fixture.requests.count == 1)
    #expect(request.url?.path == path + "/manifest")
    return request
}
#endif
