import Foundation
import Testing
@testable import KinosailPlayer

struct ContractTests {
    @Test(arguments: [
        "", "ftp://server.local", "http://example.com", "https://user:secret@example.com",
        "https://example.com/library", "https://example.com?token=secret", "https://example.com#fragment",
        "http://192.168.1.999", "http://127.1", "http://2130706433", "http://0177.0.0.1",
        "https://example.com:0", "https://example.com:65536", "https://example.com\\@elsewhere.test",
        "https://exam\nple.com", "https://" + String(repeating: "a", count: 2048)
    ])
    func rejectsInvalidServerAddresses(_ input: String) {
        #expect(throws: ClientError.self) { try ServerAddress(input) }
    }

    @Test(arguments: [
        "https://example.com", "http://localhost:8080", "http://server.local:38127",
        "http://127.0.0.1", "http://10.0.0.1", "http://172.16.0.1", "http://172.31.255.255",
        "http://192.168.1.1", "http://169.254.1.1", "http://[::1]", "http://[fd00::1]"
    ])
    func acceptsHTTPSAndExplicitLocalOrigins(_ input: String) throws {
        let address = try ServerAddress(input)
        #expect(!address.url.absoluteString.isEmpty)
    }

    @Test func normalizesOnce() throws {
        #expect(try ServerAddress("  HTTPS://EXAMPLE.COM:443/  ").url.absoluteString == "https://example.com")
    }

    @Test(arguments: [
        "https://other.example/art/1", "//other.example/art/1", "http://example.com/art/1",
        "https://example.com:8443/art/1", "https://user:secret@example.com/art/1", "/art/1#secret", ""
    ])
    func rejectsForeignMediaOrigins(_ path: String) throws {
        let server = try ServerAddress("https://example.com")
        #expect(throws: ClientError.self) { try server.mediaURL(path) }
    }

    @Test func keepsAuthenticatedResourcesOnServer() throws {
        let server = try ServerAddress("https://example.com")
        #expect(try server.mediaURL("/art/item").absoluteString == "https://example.com/art/item")
    }

    @Test(arguments: ["", "12345", "1234567", "abcdef", " 12345", "１２３４５６", "123456\n"])
    func rejectsInvalidApprovalCodes(_ code: String) {
        #expect(throws: ClientError.self) { try Input.code(code) }
    }

    @Test(arguments: ["", "session token", "token\n", "token\r", String(repeating: "x", count: 2049)])
    func rejectsInvalidCredentials(_ value: String) {
        #expect(throws: ClientError.self) { try Input.secret(value) }
    }

    @Test(arguments: [-1.0, Double.infinity, Double.nan, 31_536_001.0])
    func rejectsInvalidProgress(_ value: Double) {
        #expect(throws: ClientError.self) { try Input.position(value) }
    }

    @Test func rejectsUnknownProgressFields() {
        #expect(throws: ClientError.self) { try WatchProgress(.object(["unexpected": .bool(true)])) }
    }

    @Test func doesNotCoerceBooleansToNumbers() {
        #expect(throws: ClientError.self) { try WatchProgress(.object(["seconds": .bool(true)])) }
    }

    @Test func doesNotCoerceFractionalRevisions() {
        #expect(throws: ClientError.self) { try WatchProgress(.object(["revision": .number(1.5)])) }
    }

    @Test func rejectsUnknownJSONFields() {
        #expect(throws: ClientError.self) { try JSONValue.object(["extra": .null]).object(allowing: ["known"]) }
    }

    @Test func boundsResponseCardinality() {
        #expect(throws: ClientError.self) { try JSONValue.array([.null, .null]).array(max: 1) }
    }
}
