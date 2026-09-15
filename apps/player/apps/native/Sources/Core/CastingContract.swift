import Foundation

extension CastStart {
    var json: JSONValue {
        get throws {
            try Input.position(position)
            var value: [String: JSONValue] = ["protocol": .string(receiverProtocol.rawValue), "position": .number(position)]
            if receiverProtocol == .dlna {
                guard let deviceID else { throw ClientError.invalidInput("Choose a DLNA receiver.") }
                value["deviceId"] = .string(try Input.hex(deviceID, count: 32))
            } else if deviceID != nil { throw ClientError.invalidInput("The receiver does not match the selected protocol.") }
            if let playbackToken { value["playbackToken"] = .string(try Input.text(playbackToken, max: 8192, label: "playback token", empty: true)) }
            return .object(value)
        }
    }
}

extension CastCommand {
    var json: JSONValue {
        get throws {
            switch self {
            case .play: return .object(["action": .string("play")])
            case .pause: return .object(["action": .string("pause")])
            case .stop: return .object(["action": .string("stop")])
            case .seek(let seconds):
                try Input.position(seconds)
                return .object(["action": .string("seek"), "position": .number(seconds)])
            }
        }
    }
}

extension CastDevice {
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["id", "name", "protocol"])
        id = try Input.hex(value.text("id", max: 32, required: true), count: 32)
        name = try Input.text(value.text("name", max: 128, required: true), max: 128, label: "receiver name")
        guard try value.text("protocol") == "dlna" else { throw ClientError.invalidResponse }
        receiverProtocol = .dlna
    }
}

extension CastSession {
    init(_ raw: JSONValue, server: ServerAddress) throws {
        let value = try raw.object(allowing: ["id", "url", "contentType", "title", "position", "duration", "expiresAt", "protocol", "tracks", "deviceId", "deviceName"])
        id = try Input.hex(value.text("id", max: 32, required: true), count: 32)
        guard let receiverProtocol = try CastProtocol(rawValue: value.text("protocol", required: true)) else { throw ClientError.invalidResponse }
        self.receiverProtocol = receiverProtocol
        url = try server.mediaURL(value.text("url", max: 4096, required: true))
        guard ["/cast/\(id)/media", "/cast/\(id)/hls/index.m3u8"].contains(url.path),
              let query = URLComponents(url: url, resolvingAgainstBaseURL: false)?.queryItems,
              query.count == 1, query[0].name == "ticket", let ticket = query[0].value else { throw ClientError.invalidResponse }
        _ = try Input.hex(ticket, count: 64)
        contentType = try value.text("contentType", max: 128, required: true)
        guard contentType.range(of: "\\A(?:video/[a-z0-9.+-]+|audio/[a-z0-9.+-]+|application/vnd\\.apple\\.mpegurl)\\z", options: .regularExpression) != nil else { throw ClientError.invalidResponse }
        title = try Input.text(value.text("title", max: 1024, required: true), max: 1024, label: "title")
        position = try value.requiredNumber("position"); duration = try value.requiredNumber("duration")
        guard duration == 0 || position <= duration else { throw ClientError.invalidResponse }
        expiresAt = try Input.date(value.text("expiresAt", max: 64, required: true))
        guard expiresAt > Date(), expiresAt.timeIntervalSinceNow <= 25 * 3600 else { throw ClientError.invalidResponse }
        let base = url, sessionID = id
        tracks = try value.required("tracks").array(max: 64).enumerated().map { index, raw in
            let track = try raw.object(allowing: ["id", "url", "label", "language", "default"])
            guard try track.requiredNumber("id", max: 64, integer: true) == Double(index + 1) else { throw ClientError.invalidResponse }
            let url = try server.mediaURL(track.text("url", max: 4096, required: true))
            guard url.path == "/cast/\(sessionID)/subtitles/\(index + 1)", url.query == base.query else { throw ClientError.invalidResponse }
            let language = try track.text("language", max: 32)
            return try Track(id: index + 1, url: url, label: Input.text(track.text("label", max: 512, required: true), max: 512, label: "subtitle label"), language: language.isEmpty ? "und" : Input.text(language, max: 32, label: "subtitle language"), isDefault: track.flag("default"))
        }
        if receiverProtocol == .dlna {
            deviceID = try Input.hex(value.text("deviceId", max: 32, required: true), count: 32)
            deviceName = try Input.text(value.text("deviceName", max: 128, required: true), max: 128, label: "receiver name")
        } else {
            guard value["deviceId"] == nil, value["deviceName"] == nil else { throw ClientError.invalidResponse }
            deviceID = nil; deviceName = nil
        }
    }
}

extension CastStatus {
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["state", "position", "duration"])
        guard let state = try State(rawValue: value.text("state", max: 16, required: true)) else { throw ClientError.invalidResponse }
        self.state = state
        position = try value.requiredNumber("position"); duration = try value.requiredNumber("duration")
        guard duration == 0 || position <= duration + 2 else { throw ClientError.invalidResponse }
    }
}
