import Foundation
import Synchronization
import Testing
@testable import KinosailPlayer

@Suite(.serialized) struct QuickConnectPollingTests {
    private static let pending = FixtureURLProtocol.Entry(data: Data("{\"status\":\"pending\"}".utf8), status: 202, headers: [:])
    private static let approved = FixtureURLProtocol.Entry(data: Data("{\"token\":\"approved-token\",\"expiresIn\":3600}".utf8), status: 201, headers: [:])

    private static func interrupted(_ code: URLError.Code) -> FixtureURLProtocol.Entry {
        var entry = FixtureURLProtocol.Entry(data: Data(), status: 202, headers: [:])
        entry.failure = code
        return entry
    }

    private func fixture(_ sequence: [FixtureURLProtocol.Entry]) throws -> HTTPFixture {
        let fixture = try HTTPFixture(body: "{\"status\":\"pending\"}", status: 202)
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.sequence = sequence }
        return fixture
    }

    @Test(arguments: [URLError.Code.networkConnectionLost, .cancelled, .timedOut, .notConnectedToInternet])
    func approvalSurvivesAPollInterruptedWhileTheBrowserIsInFront(_ code: URLError.Code) async throws {
        let fixture = try fixture([Self.pending, Self.interrupted(code), Self.approved])
        defer { fixture.remove() }
        let token = try await fixture.client.awaitQuickConnect(secret: "pending-secret", until: Date().addingTimeInterval(30), interval: .milliseconds(10))
        #expect(token == "approved-token")
        #expect(fixture.requests.allSatisfy { $0.url?.path == "/api/v1/quick-connect/token" })
        #expect(fixture.requests.count == 3)
    }

    @Test func cancelledCodeStopsPollingAtOnce() async throws {
        let gone = FixtureURLProtocol.Entry(data: Data("{}".utf8), status: 404, headers: [:])
        let fixture = try fixture([Self.pending, gone, Self.approved])
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) {
            try await fixture.client.awaitQuickConnect(secret: "pending-secret", until: Date().addingTimeInterval(30), interval: .milliseconds(10))
        }
        #expect(fixture.requests.count == 2)
    }

    @Test func malformedPendingStopsPollingAtOnce() async throws {
        let malformed = FixtureURLProtocol.Entry(data: Data("{\"status\":\"approved\"}".utf8), status: 202, headers: [:])
        let fixture = try fixture([malformed, Self.approved])
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) {
            try await fixture.client.awaitQuickConnect(secret: "pending-secret", until: Date().addingTimeInterval(30), interval: .milliseconds(10))
        }
        #expect(fixture.requests.count == 1)
    }

    @Test func pollingEndsAtTheDeadlineWhenTheServerStaysUnreachable() async throws {
        let fixture = try fixture(Array(repeating: Self.interrupted(.cannotConnectToHost), count: 200))
        defer { fixture.remove() }
        let started = ContinuousClock.now
        await #expect(throws: ClientError.invalidInput("That code expired. Connect again to get a new code.")) {
            try await fixture.client.awaitQuickConnect(secret: "pending-secret", until: Date().addingTimeInterval(0.3), interval: .milliseconds(10))
        }
        #expect(ContinuousClock.now - started < .seconds(5))
    }

    @Test func viewerCancellationStopsPolling() async throws {
        let fixture = try fixture([])
        defer { fixture.remove() }
        let polling = Task { try await fixture.client.awaitQuickConnect(secret: "pending-secret", until: Date().addingTimeInterval(30), interval: .milliseconds(50)) }
        try await Task.sleep(for: .milliseconds(120))
        polling.cancel()
        await #expect(throws: CancellationError.self) { try await polling.value }
        let count = fixture.requests.count
        try await Task.sleep(for: .milliseconds(200))
        #expect(fixture.requests.count == count)
    }
}
