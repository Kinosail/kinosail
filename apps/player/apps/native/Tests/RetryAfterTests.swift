import Foundation
import Testing
@testable import KinosailPlayer

struct RetryAfterTests {
    @Test(arguments: [("5", 5), ("1", 1), ("0", 1), ("60", 60), ("3600", 60)])
    func unavailableReadWithRetryAfterWaitsForTheServer(_ header: String, _ seconds: Int) async throws {
        let fixture = try HTTPFixture(body: "{\"error\":\"Library scan is in progress\"}", status: 503, headers: ["Retry-After": header])
        defer { fixture.remove() }
        await #expect(throws: ClientError.retryAfter(seconds)) { try await fixture.client.library() }
        #expect(fixture.requests.count == 1, "A deferred read must wait for the Server delay instead of quick retries")
        #expect(ClientError.retryAfter(seconds).permitsOfflineRestore)
    }

    @Test(arguments: ["soon", "-5", "1.5", " 5", "12345", "Sat, 17 Oct 2026 07:28:00 GMT"])
    func malformedRetryAfterKeepsTheUnavailableFailure(_ header: String) async throws {
        let fixture = try HTTPFixture(body: "", status: 503, headers: ["Retry-After": header])
        defer { fixture.remove() }
        await #expect(throws: ClientError.http(503)) { try await fixture.client.library() }
    }

    @Test func retryAfterOnOtherStatusesKeepsTheStatus() async throws {
        let fixture = try HTTPFixture(body: "", status: 429, headers: ["Retry-After": "5"])
        defer { fixture.remove() }
        await #expect(throws: ClientError.http(429)) { try await fixture.client.library() }
    }

    @Test func deferredRetryCarriesOnlyTheServerDelay() {
        let retry = DeferredRetry(ClientError.retryAfter(7))
        #expect(retry?.seconds == 7)
        #expect(retry?.message == "Your Server is still getting ready. Kinosail will try again shortly.")
        #expect(DeferredRetry(ClientError.http(503)) == nil)
        #expect(DeferredRetry(ClientError.unavailable) == nil)
    }
}
