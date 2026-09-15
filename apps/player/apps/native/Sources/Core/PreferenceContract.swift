import Foundation

extension PlaybackPreferences {
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["rate", "audioLanguage", "subtitleLanguage", "audioTrack", "subtitleTrack", "nightMode", "dialogueBoost", "volumeBoost"])
        self.init()
        rate = try value.requiredNumber("rate", max: 3)
        volumeBoost = try value.requiredNumber("volumeBoost", max: 2)
        guard rate >= 0.5, volumeBoost >= 1 else { throw ClientError.invalidResponse }
        audioLanguage = try Self.language(value.text("audioLanguage", max: 32, required: true), subtitle: false)
        subtitleLanguage = try Self.language(value.text("subtitleLanguage", max: 32, required: true), subtitle: true)
        audioTrack = try Input.text(value.text("audioTrack", max: 256), max: 256, label: "audio track", empty: true)
        subtitleTrack = try Input.text(value.text("subtitleTrack", max: 256), max: 256, label: "subtitle track", empty: true)
        nightMode = try value.flag("nightMode")
        dialogueBoost = try value.flag("dialogueBoost")
    }

    static func language(_ value: String, subtitle: Bool) throws -> String {
        guard value.utf8.count <= 32, value == "auto" || subtitle && value == "off" ||
                value.range(of: "\\A[a-z]{2,3}(?:-[A-Za-z0-9]{2,8}){0,3}\\z", options: .regularExpression) != nil else {
            throw ClientError.invalidInput("Choose a valid language.")
        }
        return value
    }

    var json: JSONValue {
        .object(["rate": .number(rate), "audioLanguage": .string(audioLanguage), "subtitleLanguage": .string(subtitleLanguage),
                 "audioTrack": .string(audioTrack), "subtitleTrack": .string(subtitleTrack), "nightMode": .bool(nightMode),
                 "dialogueBoost": .bool(dialogueBoost), "volumeBoost": .number(volumeBoost)])
    }
}

extension MediaPreferences {
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["playback", "autoDownloadNext", "removeWatched", "downloadLimitGiB", "wifiOnly", "readerFontSize", "readerTheme"])
        self.init()
        playback = try PlaybackPreferences(value.required("playback"))
        autoDownloadNext = Int(try value.requiredNumber("autoDownloadNext", max: 3, integer: true))
        removeWatched = try value.flag("removeWatched")
        downloadLimitGiB = Int(try value.requiredNumber("downloadLimitGiB", max: 8_388_607, integer: true))
        wifiOnly = try value.flag("wifiOnly")
        readerFontSize = Int(try value.requiredNumber("readerFontSize", max: 32, integer: true))
        guard readerFontSize >= 16, let theme = try ReaderTheme(rawValue: value.text("readerTheme", max: 16, required: true)) else {
            throw ClientError.invalidResponse
        }
        readerTheme = theme
    }

    var json: JSONValue {
        .object(["playback": playback.json, "autoDownloadNext": .number(Double(autoDownloadNext)),
                 "removeWatched": .bool(removeWatched), "downloadLimitGiB": .number(Double(downloadLimitGiB)),
                 "wifiOnly": .bool(wifiOnly), "readerFontSize": .number(Double(readerFontSize)), "readerTheme": .string(readerTheme.rawValue)])
    }
}

extension ItemPlaybackPreferences {
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["playback", "overridden"])
        try self.init(playback: PlaybackPreferences(value.required("playback")), overridden: value.flag("overridden"))
    }
}

extension Bookmark {
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["id", "title", "seconds", "page", "offset"])
        let id = try Input.hex(value.text("id", max: 64, required: true), count: 64)
        let title = try Input.text(value.text("title", max: 160, required: true), max: 160, label: "bookmark name")
        let position: BookmarkPosition
        if value["seconds"] != nil {
            guard value["page"] == nil, value["offset"] == nil else { throw ClientError.invalidResponse }
            position = .playback(seconds: try value.requiredNumber("seconds"))
        } else {
            let page = Int(try value.requiredNumber("page", max: 10_000, integer: true))
            guard page >= 1 else { throw ClientError.invalidResponse }
            position = .reading(page: page, offset: try value.number("offset", max: 1))
        }
        self.init(id: id, title: title, position: position)
    }

    static func request(title: String, position: BookmarkPosition) throws -> JSONValue {
        var body: [String: JSONValue] = ["title": .string(try Input.text(title, max: 160, label: "bookmark name"))]
        switch position {
        case .playback(let seconds):
            try Input.position(seconds)
            body["seconds"] = .number(seconds)
        case .reading(let page, let offset):
            guard (1...10_000).contains(page), offset.isFinite, (0...1).contains(offset) else {
                throw ClientError.invalidInput("The reading position is invalid.")
            }
            body["page"] = .number(Double(page)); body["offset"] = .number(offset)
        }
        return .object(body)
    }
}
