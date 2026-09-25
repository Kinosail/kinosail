import Foundation

struct WatchPlayerState: Codable, Sendable, Identifiable {
    let id: String
    let device: String
    let title: String
    let subtitle: String
    let itemID: String
    let position: Double
    let duration: Double
    let playing: Bool
    let audio: Bool

    var active: Bool { !itemID.isEmpty }

    func validated() throws -> Self {
        guard (1...64).contains(id.utf8.count), (1...80).contains(device.utf8.count), title.utf8.count <= 256,
              subtitle.utf8.count <= 128, itemID.utf8.count <= 128, position.isFinite, duration.isFinite,
              position >= 0, duration >= 0, duration <= 1_000_000_000, position <= duration,
              active == !title.isEmpty else { throw WatchRemoteError.invalidMessage }
        return self
    }
}

struct WatchRemoteReply: Codable, Sendable {
    let players: [WatchPlayerState]
    let accepted: Bool
    let message: String?

    func data() throws -> Data {
        guard players.count <= 32, Set(players.map(\.id)).count == players.count else { throw WatchRemoteError.invalidMessage }
        _ = try players.map { try $0.validated() }
        let encoded = try JSONEncoder().encode(self)
        guard encoded.count <= 16_384 else { throw WatchRemoteError.invalidMessage }
        return encoded
    }

    static func parse(_ data: Data) throws -> Self {
        guard data.count <= 16_384 else { throw WatchRemoteError.invalidMessage }
        let reply = try JSONDecoder().decode(Self.self, from: data)
        guard reply.players.count <= 32, Set(reply.players.map(\.id)).count == reply.players.count else { throw WatchRemoteError.invalidMessage }
        _ = try reply.players.map { try $0.validated() }
        return reply
    }
}

enum WatchRemoteError: Error { case invalidMessage }

struct WatchRemoteRequest: Sendable {
    let target: String?
    let command: String?
    let position: Double?

    static func parse(_ data: Data) throws -> Self {
        guard data.count <= 1024,
              let value = try JSONSerialization.jsonObject(with: data) as? [String: Any] else { throw WatchRemoteError.invalidMessage }
        let keys = Set(value.keys)
        if keys == ["action"], value["action"] as? String == "list" { return Self(target: nil, command: nil, position: nil) }
        guard keys == ["action", "target"] || keys == ["action", "target", "position"],
              let command = value["action"] as? String,
              let target = value["target"] as? String, (1...64).contains(target.utf8.count),
              ["play", "pause", "backward", "forward", "previous", "next", "seek"].contains(command) else { throw WatchRemoteError.invalidMessage }
        if command == "seek" {
            guard keys.contains("position"), let number = value["position"] as? NSNumber,
                  CFGetTypeID(number) != CFBooleanGetTypeID(), number.doubleValue.isFinite,
                  (0...1_000_000_000).contains(number.doubleValue) else { throw WatchRemoteError.invalidMessage }
            return Self(target: target, command: command, position: number.doubleValue)
        }
        guard !keys.contains("position") else { throw WatchRemoteError.invalidMessage }
        return Self(target: target, command: command, position: nil)
    }

    func data() throws -> Data {
        guard let target, let command else { return Data(#"{"action":"list"}"#.utf8) }
        var value: [String: Any] = ["action": command, "target": target]
        if let position { value["position"] = position }
        let data = try JSONSerialization.data(withJSONObject: value)
        _ = try Self.parse(data)
        return data
    }
}
