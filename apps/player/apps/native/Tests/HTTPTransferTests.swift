import Foundation
import Testing
@testable import KinosailPlayer

struct HTTPTransferTests {
    @Test(arguments: [1, 31, 16_384, 65_536])
    func returnsAllChunksAtTheExactLimit(_ chunkSize: Int) async throws {
        let body = String(repeating: "artwork-data", count: 8192)
        let fixture = try HTTPFixture(body: body, headers: ["Content-Length": "", "Content-Type": "image/jpeg"])
        defer { fixture.remove() }
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.chunkSize = chunkSize }
        let (data, type) = try await fixture.client.resource("/art/movie", maximum: body.utf8.count)
        #expect(data == Data(body.utf8))
        #expect(type == "image/jpeg")
        #expect(fixture.requests.count == 1)
    }

    @Test(arguments: ["", "999999"])
    func rejectsOversizedDeclaredOrStreamedBodies(_ length: String) async throws {
        let fixture = try HTTPFixture(body: String(repeating: "x", count: 1025), headers: ["Content-Length": length])
        defer { fixture.remove() }
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.chunkSize = 256 }
        await #expect(throws: ClientError.invalidResponse) { try await fixture.client.resource("/art/movie", maximum: 1024) }
    }

    @Test func failsPartialTransfersWithoutReturningPartialSuccess() async throws {
        let fixture = try HTTPFixture(body: "partial", headers: ["Content-Length": ""])
        defer { fixture.remove() }
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.failure = .networkConnectionLost }
        await #expect(throws: ClientError.unavailable) { try await fixture.client.resource("/art/movie", maximum: 1024) }
    }

    @Test func keepsConcurrentResponseBuffersSeparate() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/art/first"] = .init(data: Data(repeating: 1, count: 8192), status: 200, headers: [:], chunkSize: 31)
            $0[fixture.host]?.routes["/art/second"] = .init(data: Data(repeating: 2, count: 4096), status: 200, headers: [:], chunkSize: 17)
        }
        async let first = fixture.client.resource("/art/first", maximum: 8192)
        async let second = fixture.client.resource("/art/second", maximum: 4096)
        let result = try await (first, second)
        #expect(result.0.0 == Data(repeating: 1, count: 8192))
        #expect(result.1.0 == Data(repeating: 2, count: 4096))
    }

    @Test(arguments: [false, true])
    func cancelsHeldResponsesOnTaskCancellationOrSignOut(_ signOut: Bool) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.hold = true }
        let request = Task { try await fixture.client.request("/api/v1/me") }
        defer { request.cancel() }
        for _ in 0..<200 where fixture.requests.isEmpty { try await Task.sleep(for: .milliseconds(5)) }
        #expect(fixture.requests.count == 1)
        if signOut { await fixture.client.close() } else { request.cancel() }
        await #expect(throws: CancellationError.self) { try await request.value }
    }

    @Test func rejectsInvalidLimitsAndClosedClientsBeforeNetwork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        for maximum in [0, -1, 64 * 1024 * 1024 + 1] {
            await #expect(throws: ClientError.self) { try await fixture.client.resource("/art/movie", maximum: maximum) }
        }
        await fixture.client.close()
        await #expect(throws: CancellationError.self) { try await fixture.client.request("/api/v1/me") }
        #expect(fixture.requests.isEmpty)
    }
}
