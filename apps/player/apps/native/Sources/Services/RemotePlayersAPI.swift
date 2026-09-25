import Foundation

struct RemotePlayerCommand: Sendable {
    let action: String
    let itemID: String
    let position: Double?

    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["command", "itemId", "position"])
        action = try value.text("command", max: 16, required: true)
        itemID = try Input.id(value.text("itemId", max: 128, required: true))
        guard ["play", "pause", "backward", "forward", "previous", "next", "seek"].contains(action) else { throw ClientError.invalidResponse }
        if action == "seek" {
            guard value["position"] != nil else { throw ClientError.invalidResponse }
            position = try value.number("position", max: 1_000_000_000)
        } else {
            guard value["position"] == nil else { throw ClientError.invalidResponse }
            position = nil
        }
    }
}

extension ServerClient {
    func remotePlayers() async throws -> [WatchPlayerState] {
        let response = try await request("/api/v1/remote-players")
        let value = try response.body.object(allowing: ["players"])
        let players = try value.list("players", max: 64).map { raw -> WatchPlayerState in
            let value = try raw.object(allowing: ["id", "name", "title", "artist", "itemId", "state", "position", "duration", "audio"])
            let id = try value.text("id", max: 36, required: true)
            guard UUID(uuidString: id)?.uuidString.lowercased() == id else { throw ClientError.invalidResponse }
            let state = try value.text("state", max: 16, required: true)
            guard ["idle", "playing", "paused", "buffering"].contains(state) else { throw ClientError.invalidResponse }
            let player = WatchPlayerState(id: id, device: try value.text("name", max: 80, required: true),
                                          title: try value.text("title", max: 256), subtitle: try value.text("artist", max: 128),
                                          itemID: try value.text("itemId", max: 128), position: try value.number("position", max: 1_000_000_000),
                                          duration: try value.number("duration", max: 1_000_000_000),
                                          playing: state == "playing", audio: try value.flag("audio"))
            return try player.validated()
        }
        guard Set(players.map(\.id)).count == players.count else { throw ClientError.invalidResponse }
        return players
    }

    func updateRemotePlayer(id: String, state: WatchPlayerState) async throws -> RemotePlayerCommand? {
        guard UUID(uuidString: id)?.uuidString.lowercased() == id else { throw ClientError.invalidInput("The player ID is invalid.") }
        _ = try state.validated()
        let body: JSONValue = .object(["name": .string(state.device), "title": .string(state.title), "artist": .string(state.subtitle),
                                       "itemId": .string(state.itemID), "state": .string(state.active ? state.playing ? "playing" : "paused" : "idle"),
                                       "position": .number(state.position), "duration": .number(state.duration), "audio": .bool(state.audio)])
        let response = try await request("/api/v1/remote-players/\(id)", method: .put, body: body)
        let value = try response.body.object(allowing: ["command"])
        guard let command = value["command"] else { throw ClientError.invalidResponse }
        if case .null = command { return nil }
        return try RemotePlayerCommand(command)
    }

    func commandRemotePlayer(id: String, itemID: String, command: WatchRemoteRequest) async throws {
        guard UUID(uuidString: id)?.uuidString.lowercased() == id, command.target == id, let action = command.command else { throw ClientError.invalidInput("The player command is invalid.") }
        let validItemID = try Input.id(itemID)
        var body: [String: JSONValue] = ["command": .string(action), "itemId": .string(validItemID)]
        if let position = command.position { body["position"] = .number(position) }
        _ = try await request("/api/v1/remote-players/\(id)/commands", method: .post, body: .object(body), expected: [202])
    }
}
