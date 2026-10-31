import CryptoKit
import Foundation

struct SourceRef: Codable, Hashable, Sendable {
    let source: String
    let item: String
}

struct DevicePerson: Codable, Hashable, Sendable {
    let name: String
    let role: String
    let image: SourceRef?
    var id: String?
}

struct DeviceUserState: Codable, Hashable, Sendable {
    var seconds: Double = 0
    var watched = false
    var favorite = false
    var lastPlayed: Date?
}

struct DeviceItem: Codable, Sendable {
    enum Kind: String, Codable, Sendable { case movie, episode, series }

    var id = ""
    let kind: Kind
    let ref: SourceRef
    var title: String
    var sortTitle: String
    var year = ""
    var plot = ""
    var tagline = ""
    var rating = ""
    var genres = ""
    var studio = ""
    var director = ""
    var seriesTitle = ""
    var seriesRef: SourceRef?
    var seriesProviders: [String: String] = [:]
    var season = 0
    var episode = 0
    var added: Date?
    var duration: Double = 0
    var container = ""
    var size: Int64 = 0
    var width = 0
    var providers: [String: String] = [:]
    var hasPrimary = false
    var hasBackdrop = false
    var seriesHasPrimary = false
    var seriesHasBackdrop = false
    var state = DeviceUserState()
    var cast: [DevicePerson] = []
    var versions: [SourceRef] = []

    var mergeKey: String {
        switch kind {
        case .movie:
            for name in ["tmdb", "imdb"] { if let value = providers[name], !value.isEmpty { return "movie:\(name):\(value)" } }
            return "movie:source:\(ref.source):\(ref.item)"
        case .series:
            return "show:" + DeviceItem.seriesKey(providers: providers, ref: ref)
        case .episode:
            let show = seriesRef.map { DeviceItem.seriesKey(providers: seriesProviders, ref: $0) } ?? "source:\(ref.source):\(ref.item)"
            guard season > 0 || episode > 0 else { return "episode:source:\(ref.source):\(ref.item)" }
            return "episode:\(show):\(season):\(episode)"
        }
    }

    var showKey: String? {
        switch kind {
        case .series: "show:" + DeviceItem.seriesKey(providers: providers, ref: ref)
        case .episode: seriesRef.map { "show:" + DeviceItem.seriesKey(providers: seriesProviders, ref: $0) }
        case .movie: nil
        }
    }

    static func seriesKey(providers: [String: String], ref: SourceRef) -> String {
        for name in ["tvdb", "tmdb", "imdb"] { if let value = providers[name], !value.isEmpty { return "\(name):\(value)" } }
        return "source:\(ref.source):\(ref.item)"
    }

    static func identifier(_ key: String) -> String {
        SHA256.hash(data: Data(key.utf8)).prefix(8).map { String(format: "%02x", $0) }.joined()
    }

    static func sortable(_ title: String) -> String {
        let lowered = title.trimmingCharacters(in: .whitespaces)
        for article in ["The ", "A ", "An "] where lowered.hasPrefix(article) && lowered.count > article.count {
            return String(lowered.dropFirst(article.count))
        }
        return lowered
    }
}

struct DevicePlaybackInfo: Sendable {
    struct Stream: Sendable {
        let index: Int
        let codec: String
        let language: String
        let title: String
        let isDefault: Bool
        let isForced: Bool
        let isExternal: Bool
        let channels: Int
        let deliveryURL: URL?
    }

    let ref: SourceRef
    let mediaSourceID: String
    let container: String
    let bitrate: Int
    let duration: Double
    let size: Int64
    let videoCodec: String
    let videoProfile: String
    let videoRange: String
    let width: Int
    let height: Int
    let audio: [Stream]
    let subtitles: [Stream]
    let chapters: [(title: String, start: Double)]
    let markers: [(type: String, start: Double, end: Double)]
    let direct: URLRequest
    let transcode: URLRequest?
    let directPlayable: Bool
    let session: String
}

protocol MediaSourceClient: Sendable {
    var account: SourceAccount { get }
    func topLevel() async throws -> [DeviceItem]
    func episodes(series: String) async throws -> [DeviceItem]
    func item(_ id: String) async throws -> DeviceItem
    func inProgress() async throws -> [DeviceItem]
    func search(_ query: String) async throws -> [DeviceItem]
    func image(_ ref: SourceRef, kind: DeviceImageKind, width: Int) throws -> URLRequest
    func playback(_ id: String, capabilities: DeviceCapabilities, start: Double) async throws -> DevicePlaybackInfo
    func report(_ id: String, seconds: Double, watched: Bool, event: DeviceProgressEvent, session: String) async throws
    func setFavorite(_ id: String, favorite: Bool) async throws
    func credits(person: String) async throws -> [DeviceItem]
    func setPosition(_ id: String, seconds: Double, watched: Bool) async throws
}

enum DeviceImageKind: String, Sendable { case primary, backdrop, thumb, person }

enum DeviceProgressEvent: Sendable { case start, progress, stop }

struct DeviceCapabilities: Sendable {
    var videoCodecs: Set<String> = ["h264"]
    var audioCodecs: Set<String> = ["aac", "mp3", "ac3", "eac3"]
    var hdr: Set<String> = ["sdr"]
    var maxAudioChannels = 2

    init() {}

    init(query: [String: String]) {
        func list(_ name: String) -> Set<String> {
            Set((query[name] ?? "").split(separator: ",").map { $0.lowercased() }.filter { $0.count <= 16 }.prefix(8))
        }
        let video = list("videoCodecs"), audio = list("audioCodecs"), hdr = list("hdrFormats")
        if !video.isEmpty { videoCodecs = video }
        if !audio.isEmpty { audioCodecs = audio }
        if !hdr.isEmpty { self.hdr = hdr }
        maxAudioChannels = Int(query["maxAudioChannels"] ?? "").map { min(8, max(1, $0)) } ?? 2
    }

    var convertedAudioCodecs: [String] {
        let codecs = ["aac", "ac3", "eac3"].filter { $0 == "aac" || audioCodecs.contains($0) }
        return maxAudioChannels >= 6 && codecs.contains("eac3") ? codecs.reversed() : codecs
    }
}
