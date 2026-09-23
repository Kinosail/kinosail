import Foundation

extension ServerClient {
    func supporterCollection() async throws -> SupporterCollection {
        try SupporterCollection(await request("/api/v1/supporter/collection").body)
    }
    func setSupporterVisibility(_ visible: Bool) async throws {
        _ = try await request("/api/v1/supporter/display", method: .put,
            body: .object(["display": .string(visible ? "automatic" : "hidden")]))
    }
}
