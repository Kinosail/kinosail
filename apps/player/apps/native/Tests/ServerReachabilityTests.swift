import Foundation
import Testing
@testable import KinosailPlayer

struct ServerReachabilityTests {
    @Test func reportsRealTransportFailureAndRecovery() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        var events = await fixture.client.connectionUpdates().makeAsyncIterator()
        #expect(await events.next() == .unknown)
        _ = try await fixture.client.request("/api/v1/me")
        #expect(await events.next() == .reachable)
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.failure = .notConnectedToInternet }
        await #expect(throws: ClientError.unavailable) { try await fixture.client.request("/api/v1/me") }
        #expect(await events.next() == .unreachable)
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.failure = nil }
        _ = try await fixture.client.request("/api/v1/me")
        #expect(await events.next() == .reachable)
        await fixture.client.close()
        #expect(await events.next() == nil)
    }

    @Test(arguments: [401, 403, 404, 429, 500, 503])
    func doesNotConfuseAuthorizationOrMissingTitlesWithOffline(_ status: Int) async throws {
        let fixture = try HTTPFixture(body: "{}", status: status)
        defer { fixture.remove() }
        await #expect(throws: ClientError.http(status)) { try await fixture.client.request("/api/v1/me") }
        #expect(await fixture.client.reachability == (status >= 500 ? .unreachable : .reachable))
        await fixture.client.close()
    }

    @Test func cancellationAndInvalidInputDoNotReportAnOutage() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        _ = try await fixture.client.request("/api/v1/me")
        let cancelled = Task {
            withUnsafeCurrentTask { $0?.cancel() }
            return try await fixture.client.request("/api/v1/me")
        }
        await #expect(throws: CancellationError.self) { try await cancelled.value }
        await #expect(throws: ClientError.self) { try await fixture.client.request("https://elsewhere.invalid") }
        #expect(await fixture.client.reachability == .reachable)
        #expect(fixture.requests.count == 1)
        await fixture.client.close()
    }

    @Test func lateResultsCannotOverwriteANewerConnectionObservation() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await fixture.client.recordConnection(.unreachable, sequence: 2)
        await fixture.client.recordConnection(.reachable, sequence: 1)
        #expect(await fixture.client.reachability == .unreachable)
        await fixture.client.recordConnection(.reachable, sequence: 3)
        #expect(await fixture.client.reachability == .reachable)
        await fixture.client.close()
    }

    @Test func observersAndProfilesRemainIsolated() async throws {
        let first = try HTTPFixture(body: "{}")
        let second = try HTTPFixture(body: "{}")
        defer { first.remove(); second.remove() }
        var previous = await first.client.connectionUpdates().makeAsyncIterator()
        #expect(await previous.next() == .unknown)
        var current = await first.client.connectionUpdates().makeAsyncIterator()
        #expect(await previous.next() == nil)
        #expect(await current.next() == .unknown)
        let cancelled = Task {
            withUnsafeCurrentTask { $0?.cancel() }
            var events = await first.client.connectionUpdates().makeAsyncIterator()
            return await events.next()
        }
        #expect(await cancelled.value == nil)
        await first.client.recordConnection(.unreachable, sequence: 1)
        #expect(await current.next() == .unreachable)
        #expect(await second.client.reachability == .unknown)
        await first.client.close(); await second.client.close()
    }
}
