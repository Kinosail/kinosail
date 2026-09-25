import Foundation

/// Playback controls are remembered on this device for the connected Viewer Profile.
struct DevicePlaybackChoices: Equatable {
    var rate: Double?
    var audioLanguage: String?
    var audioTrack: String?
    var subtitleLanguage: String?
    var subtitleTrack: String?

    static func key(scope: String) -> String { "kinosail.playback.device.v1.\(scope)" }

    init() {}

    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["rate", "audioLanguage", "audioTrack", "subtitleLanguage", "subtitleTrack"])
        if value["rate"] != nil {
            let number = try value.number("rate", max: 3)
            guard number >= 0.5 else { throw ClientError.invalidResponse }
            rate = number
        }
        if value["audioLanguage"] != nil { audioLanguage = try PlaybackPreferences.language(value.text("audioLanguage", max: 32, required: true), subtitle: false) }
        if value["subtitleLanguage"] != nil { subtitleLanguage = try PlaybackPreferences.language(value.text("subtitleLanguage", max: 32, required: true), subtitle: true) }
        if value["audioTrack"] != nil { audioTrack = try Input.text(value.text("audioTrack", max: 256), max: 256, label: "audio track", empty: true) }
        if value["subtitleTrack"] != nil { subtitleTrack = try Input.text(value.text("subtitleTrack", max: 256), max: 256, label: "subtitle track", empty: true) }
        guard audioTrack == nil || audioLanguage != nil,
              subtitleTrack == nil || subtitleLanguage != nil,
              subtitleLanguage != "off" || subtitleTrack == nil || subtitleTrack == "" else { throw ClientError.invalidResponse }
    }

    func apply(to base: PlaybackPreferences) -> PlaybackPreferences {
        var result = base
        if let rate { result.rate = rate }
        if let audioLanguage { result.audioLanguage = audioLanguage; result.audioTrack = audioTrack ?? "" }
        if let subtitleLanguage { result.subtitleLanguage = subtitleLanguage; result.subtitleTrack = subtitleTrack ?? "" }
        return result
    }

    static func load(scope: String, from defaults: UserDefaults = .standard) -> Self {
        guard let data = defaults.data(forKey: key(scope: scope)), data.count <= 1024,
              let raw = try? StrictJSON.decode(data, maximum: 1024), let choices = try? Self(raw) else { return Self() }
        return choices
    }

    func save(scope: String, to defaults: UserDefaults = .standard) {
        var value: [String: JSONValue] = [:]
        if let rate { value["rate"] = .number(rate) }
        if let audioLanguage { value["audioLanguage"] = .string(audioLanguage) }
        if let audioTrack { value["audioTrack"] = .string(audioTrack) }
        if let subtitleLanguage { value["subtitleLanguage"] = .string(subtitleLanguage) }
        if let subtitleTrack { value["subtitleTrack"] = .string(subtitleTrack) }
        if let data = try? JSONEncoder().encode(JSONValue.object(value)), data.count <= 1024 { defaults.set(data, forKey: Self.key(scope: scope)) }
    }
}
