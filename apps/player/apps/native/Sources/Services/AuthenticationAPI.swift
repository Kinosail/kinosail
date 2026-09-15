import Foundation

extension ServerClient {
    func startQuickConnect(device: String) async throws -> ConnectChallenge {
        let name = try Input.text(device, max: 80, label: "device name")
        return try ConnectChallenge(await request("/api/v1/quick-connect", method: .post,
                                                 body: .object(["device": .string(name)]), expected: [201]).body)
    }

    func pollQuickConnect(secret: String) async throws -> String? {
        let secret = try Input.secret(secret)
        let response = try await request("/api/v1/quick-connect/token", method: .post,
                                         body: .object(["secret": .string(secret)]), expected: [201, 202, 404])
        if response.status == 404 { throw ClientError.invalidInput("That code expired or was cancelled. Connect again to get a new code.") }
        if response.status == 202 {
            let value = try response.body.object(allowing: ["status"])
            guard try value.text("status", required: true) == "pending" else { throw ClientError.invalidResponse }
            return nil
        }
        let value = try response.body.object(allowing: ["token", "expiresIn"])
        _ = try value.requiredNumber("expiresIn", max: 315_360_000, integer: true)
        return try Input.secret(value.text("token", max: 512, required: true))
    }

    func cancelQuickConnect(secret: String) async throws {
        _ = try await request("/api/v1/quick-connect/cancel", method: .post,
                              body: .object(["secret": .string(try Input.secret(secret))]), expected: [204])
    }

    func viewer() async throws -> Viewer {
        let viewer = try Viewer(await request("/api/v1/me").body)
        try associate(viewer)
        return viewer
    }

    func previewApproval(code: String) async throws -> DeviceApproval {
        let code = try Input.code(code)
        let approval = try DeviceApproval(await request("/api/v1/quick-connect/\(code)").body)
        guard approval.code == code, approval.expires > Date() else { throw ClientError.invalidResponse }
        return approval
    }

    func approveDevice(code: String) async throws {
        _ = try await request("/api/v1/quick-connect/\(Input.code(code))", method: .post, expected: [204])
    }

    func signOut() async throws {
        await discardMediaCache()
        _ = try await request("/api/v1/session", method: .delete, expected: [204])
    }
}
