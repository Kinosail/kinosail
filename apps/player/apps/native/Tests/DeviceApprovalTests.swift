import Foundation
import Testing
@testable import KinosailPlayer

struct DeviceApprovalTests {
    @Test(arguments: [false, true])
    func manualAndScannedCodesPreviewBeforeApproval(scanned: Bool) async throws {
        let expires = Date().addingTimeInterval(300).ISO8601Format()
        let fixture = try HTTPFixture(body: """
        {"code":"123456","device":"Living Room TV","expiresAt":"\(expires)"}
        """)
        defer { fixture.remove() }
        var code = "123456"
        if scanned {
            let server = try ServerAddress("https://\(fixture.host)")
            var link = URLComponents()
            link.scheme = "kinosail"
            link.host = "approve"
            link.queryItems = [URLQueryItem(name: "server", value: server.url.absoluteString),
                               URLQueryItem(name: "code", value: code)]
            code = try ApprovalLink.code(#require(link.url), server: server)
        }

        let preview = try await fixture.client.previewApproval(code: code)
        #expect(preview.device == "Living Room TV")
        #expect(fixture.requests.map(\.httpMethod) == ["GET"])
        #expect(fixture.requests.first?.url?.path == "/api/v1/quick-connect/123456")

        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/api/v1/quick-connect/123456"] = .init(data: Data(), status: 204, headers: [:])
        }
        try await fixture.client.approveDevice(code: preview.code)
        #expect(fixture.requests.map(\.httpMethod) == ["GET", "POST"])
        #expect(fixture.requests.allSatisfy { $0.url?.path == "/api/v1/quick-connect/123456" })
    }

    @Test(arguments: ["expired", "mismatched"])
    func rejectsUnavailableApprovalPreview(reason: String) async throws {
        let expires = Date().addingTimeInterval(reason == "expired" ? -300 : 300).ISO8601Format()
        let code = reason == "mismatched" ? "654321" : "123456"
        let fixture = try HTTPFixture(body: """
        {"code":"\(code)","device":"Living Room TV","expiresAt":"\(expires)"}
        """)
        defer { fixture.remove() }
        await #expect(throws: ClientError.invalidResponse) { try await fixture.client.previewApproval(code: "123456") }
        #expect(fixture.requests.map(\.httpMethod) == ["GET"])
    }
}
