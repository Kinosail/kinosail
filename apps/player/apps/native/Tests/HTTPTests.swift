import Foundation
import Synchronization
import Testing
@testable import KinosailPlayer

struct HTTPTests {
    @Test(arguments: ["", "{", "{} trailing", "{\"x\":1,\"x\":2}", "{\"x\":1,\"\\u0078\":2}",
                      "[1,]", "{\"x\":}", "[true false]", "1e999", "NaN", "01", "[\"unterminated]"])
    func rejectsMalformedAndAmbiguousJSON(_ raw: String) {
        #expect(throws: ClientError.self) { try StrictJSON.decode(Data(raw.utf8)) }
    }

    @Test func rejectsExcessiveNestingAndCardinality() {
        let nested = String(repeating: "[", count: 34) + "0" + String(repeating: "]", count: 34)
        #expect(throws: ClientError.self) { try StrictJSON.decode(Data(nested.utf8)) }
        let large = "[" + Array(repeating: "0", count: 16_385).joined(separator: ",") + "]"
        #expect(throws: ClientError.self) { try StrictJSON.decode(Data(large.utf8)) }
        #expect(throws: ClientError.self) { try StrictJSON.decode(Data("{}".utf8), maximum: 1) }
    }

    @Test(arguments: ["", "bad\nname", String(repeating: "x", count: 81)])
    func rejectsDeviceBeforeNetwork(_ name: String) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.startQuickConnect(device: name) }
        #expect(fixture.requests.isEmpty)
    }

    @Test func boundsAllCatalogInputsBeforeNetwork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.library(offset: -1) }
        await #expect(throws: ClientError.self) { try await fixture.client.library(offset: 1_000_001) }
        await #expect(throws: ClientError.self) { try await fixture.client.library(limit: 201) }
        await #expect(throws: ClientError.self) { try await fixture.client.library(query: String(repeating: "x", count: 513)) }
        await #expect(throws: ClientError.self) { try await fixture.client.item(id: "../private") }
        await #expect(throws: ClientError.self) { try await fixture.client.collection(name: "../private") }
        await #expect(throws: ClientError.self) { try await fixture.client.approveDevice(code: "12345") }
        await #expect(throws: ClientError.self) { try await fixture.client.request("https://elsewhere.example/api/v1/me") }
        #expect(fixture.requests.isEmpty)
    }

    @Test func preservesPendingAndCreatedStatusAndAuthentication() async throws {
        let pending = try HTTPFixture(body: "{\"status\":\"pending\"}", status: 202)
        defer { pending.remove() }
        #expect(try await pending.client.pollQuickConnect(secret: "pending-secret") == nil)
        let issued = try HTTPFixture(body: "{\"token\":\"new-token\",\"expiresIn\":3600}", status: 201)
        defer { issued.remove() }
        #expect(try await issued.client.pollQuickConnect(secret: "pending-secret") == "new-token")
        #expect(issued.requests.first?.value(forHTTPHeaderField: "Authorization") == "Bearer fixture-token")
        #expect(issued.requests.first?.url?.query == nil)
    }

    @Test func preservesEmpty204AndRejectsRedirect() async throws {
        let empty = try HTTPFixture(body: "", status: 204)
        defer { empty.remove() }
        try await empty.client.cancelQuickConnect(secret: "pending-secret")
        let redirect = try HTTPFixture(body: "", status: 302, headers: ["Location": "https://elsewhere.example"])
        defer { redirect.remove() }
        await #expect(throws: ClientError.http(302)) { try await redirect.client.viewer() }
        #expect(redirect.requests.count == 1)
        #expect(redirect.requests.first?.url?.host == redirect.host)
    }

    @Test(arguments: ["{\"status\":\"approved\"}", "{\"status\":true}", "{\"status\":\"pending\",\"extra\":1}"])
    func rejectsMalformedPending(_ body: String) async throws {
        let fixture = try HTTPFixture(body: body, status: 202)
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.pollQuickConnect(secret: "pending-secret") }
    }

    @Test func rejectsOversizedResponsesAndMismatchedIdentity() async throws {
        let huge = try HTTPFixture(body: String(repeating: "x", count: 2 * 1024 * 1024 + 1))
        defer { huge.remove() }
        await #expect(throws: ClientError.self) { try await huge.client.viewer() }
        let mismatch = try HTTPFixture(body: "{\"item\":{\"id\":\"other\",\"kind\":\"video\",\"title\":\"Other\"},\"listed\":false,\"profileId\":\"viewer\"}")
        defer { mismatch.remove() }
        await #expect(throws: ClientError.self) { try await mismatch.client.item(id: "requested") }
    }

    @Test func acceptsCurrentLibraryProjection() async throws {
        let fixture = try HTTPFixture(body: """
        {"items":[{"id":"movie","kind":"video","title":"Movie","sortTitle":"Movie","stream":"/media/movie","download":"/download/movie","cast":[],"track":1,"subtitles":0,"size":32,"progress":{}}],"view":"all","sort":"title","query":"","letter":"","letters":[{"label":"M","offset":0,"count":1}],"total":1,"offset":0,"limit":60}
        """)
        defer { fixture.remove() }
        let page = try await fixture.client.library()
        #expect(page.items.first?.title == "Movie")
        #expect(page.letters.first?.label == "M")
    }
}

struct HTTPFixture: Sendable {
    let host: String
    let client: ServerClient
    private let ownedCacheDirectory: URL?
    var requests: [URLRequest] { FixtureURLProtocol.entries.withLock { $0[host]?.requests ?? [] } }
    init(body: String, status: Int = 200, headers: [String: String] = [:], viewer: Viewer? = nil, cacheDirectory: URL? = nil) throws {
        try self.init(data: Data(body.utf8), status: status, headers: headers, viewer: viewer, cacheDirectory: cacheDirectory)
    }
    init(data: Data, status: Int = 200, headers: [String: String] = [:], viewer: Viewer? = nil, cacheDirectory: URL? = nil) throws {
        let host = UUID().uuidString.lowercased() + ".example.invalid"
        self.host = host
        ownedCacheDirectory = cacheDirectory == nil ? FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString) : nil
        FixtureURLProtocol.entries.withLock {
            $0[host] = .init(data: data, status: status, headers: headers)
        }
        client = try ServerClient(server: ServerAddress("https://\(host)"), token: "fixture-token", viewer: viewer,
                                  protocolClasses: [FixtureURLProtocol.self], cacheDirectory: cacheDirectory ?? ownedCacheDirectory)
    }
    func remove() {
        FixtureURLProtocol.entries.withLock { _ = $0.removeValue(forKey: host) }
        if let ownedCacheDirectory { try? FileManager.default.removeItem(at: ownedCacheDirectory) }
    }
}

final class FixtureURLProtocol: URLProtocol, @unchecked Sendable {
    struct Entry: Sendable {
        let data: Data
        let status: Int
        let headers: [String: String]
        var requests: [URLRequest] = []
        var routes: [String: Entry] = [:]
        var hold = false
        var chunkSize: Int?
        var failure: URLError.Code?
    }
    static let entries = Mutex<[String: Entry]>([:])
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        guard let url = request.url, let host = url.host else { return }
        let entry = Self.entries.withLock { values -> Entry? in
            values[host]?.requests.append(request)
            return values[host]?.routes[url.path] ?? values[host]
        }
        guard let entry else { client?.urlProtocol(self, didFailWithError: URLError(.unsupportedURL)); return }
        if entry.hold { return }
        let headers = ["Content-Type": "application/json", "Content-Length": String(entry.data.count)].merging(entry.headers) { _, new in new }
        guard let response = HTTPURLResponse(url: url, statusCode: entry.status, httpVersion: "HTTP/1.1", headerFields: headers) else { return }
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        let chunkSize = max(1, entry.chunkSize ?? entry.data.count)
        for start in stride(from: 0, to: entry.data.count, by: chunkSize) {
            client?.urlProtocol(self, didLoad: entry.data.subdata(in: start..<min(start + chunkSize, entry.data.count)))
        }
        if let failure = entry.failure { client?.urlProtocol(self, didFailWithError: URLError(failure)) }
        else { client?.urlProtocolDidFinishLoading(self) }
    }
    override func stopLoading() {}
}
