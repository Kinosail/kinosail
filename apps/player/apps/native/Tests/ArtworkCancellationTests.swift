import Foundation
import Synchronization
import Testing
@testable import KinosailPlayer

struct ArtworkCancellationTests {
    @Test func cancellingEveryCardStopsTheSharedNetworkRequest() async throws {
        let host = UUID().uuidString.lowercased() + ".example.invalid"
        let client = try ServerClient(server: ServerAddress("https://\(host)"), protocolClasses: [HeldArtworkProtocol.self])
        let loader = ArtworkLoader()
        defer { HeldArtworkProtocol.remove(host) }
        let first = Task { try await loader.image(path: "/art/shared", client: client) }
        try await waitForRequest(host)
        first.cancel()
        for _ in 0..<200 where HeldArtworkProtocol.stopped(host) == 0 { try await Task.sleep(for: .milliseconds(5)) }
        #expect(HeldArtworkProtocol.stopped(host) == 1)
        // Always release the held response, including on the unfixed code.
        HeldArtworkProtocol.respond(host)
        await #expect(throws: CancellationError.self) { try await first.value }
        await loader.clear()
        await client.close()
    }

    private func waitForRequest(_ host: String) async throws {
        for _ in 0..<200 where HeldArtworkProtocol.requests(host) == 0 { try await Task.sleep(for: .milliseconds(5)) }
        try #require(HeldArtworkProtocol.requests(host) == 1)
    }
}

private final class HeldArtworkProtocol: URLProtocol, @unchecked Sendable {
    private struct Entry {
        var pending: HeldArtworkProtocol?
        var requests = 0
        var stopped = 0
    }
    private static let entries = Mutex<[String: Entry]>([:])
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        guard let host = request.url?.host else { return }
        Self.entries.withLock {
            $0[host, default: Entry()].requests += 1
            $0[host]?.pending = self
        }
    }
    override func stopLoading() {
        guard let host = request.url?.host else { return }
        Self.entries.withLock {
            $0[host]?.stopped += 1
            $0[host]?.pending = nil
        }
    }
    static func requests(_ host: String) -> Int { entries.withLock { $0[host]?.requests ?? 0 } }
    static func stopped(_ host: String) -> Int { entries.withLock { $0[host]?.stopped ?? 0 } }
    static func remove(_ host: String) { entries.withLock { _ = $0.removeValue(forKey: host) } }
    static func respond(_ host: String) {
        let pending = entries.withLock { values -> HeldArtworkProtocol? in
            let pending = values[host]?.pending
            values[host]?.pending = nil
            return pending
        }
        guard let pending, let url = pending.request.url else { return }
        let data = Data(base64Encoded: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=")!
        let response = HTTPURLResponse(url: url, statusCode: 200, httpVersion: nil, headerFields: ["Content-Type": "image/png"])!
        pending.client?.urlProtocol(pending, didReceive: response, cacheStoragePolicy: .notAllowed)
        pending.client?.urlProtocol(pending, didLoad: data)
        pending.client?.urlProtocolDidFinishLoading(pending)
    }
}
