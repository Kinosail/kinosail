import Foundation

extension ServerClient {
    func scanCastDevices() async throws -> [CastDevice] {
        let value = try await request("/api/v1/cast/devices/scan", method: .post, body: .object([:])).body.object(allowing: ["devices"])
        let devices = try value.required("devices").array(max: 64).map(CastDevice.init)
        guard Set(devices.map(\.id)).count == devices.count else { throw ClientError.invalidResponse }
        return devices
    }

    func startCast(itemID: String, request input: CastStart) async throws -> CastSession {
        let id = try Input.id(itemID), body = try input.json
        let result = try CastSession(await request("/api/v1/items/\(id)/cast", method: .post, body: body, expected: [201]).body, server: server)
        guard result.receiverProtocol == input.receiverProtocol, result.deviceID == input.deviceID else {
            try? await endCast(id: result.id)
            throw ClientError.invalidResponse
        }
        return result
    }

    func castStatus(id: String) async throws -> CastStatus {
        try CastStatus(await request("/api/v1/cast/sessions/\(Input.hex(id, count: 32))").body)
    }

    func castCommand(id: String, command: CastCommand) async throws {
        let id = try Input.hex(id, count: 32), body = try command.json
        _ = try await request("/api/v1/cast/sessions/\(id)/commands", method: .post, body: body, expected: [204])
    }

    func endCast(id: String) async throws {
        _ = try await request("/api/v1/cast/sessions/\(Input.hex(id, count: 32))", method: .delete, expected: [204])
    }
}
