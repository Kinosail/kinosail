import Foundation

struct ItemDetail: Sendable {
    let item: MediaItem
    let listed: Bool
}

// These are UI-facing contracts. Implement the wire decoders beside each API
// operation; do not expose unvalidated remote or persisted JSON to a screen.
struct HomeSnapshot: Sendable {
    let viewer: Viewer
    let continueWatching: [MediaItem]
    let recent: [MediaItem]
}

struct Album: Identifiable, Hashable, Sendable {
    let id: String
    let title: String
    let artist: String
    let artwork: String
}

struct AlbumDetail: Sendable {
    let id: String
    let title: String
    let artist: String
    let tracks: [MediaItem]
}

struct Chapter: Identifiable, Hashable, Sendable {
    let title: String
    let start: Double
    let end: Double
    var id: Double { start }
}

struct PlaybackSource: Sendable {
    let direct: URL?
    let contentType: String
    let duration: Double
    let start: Double
    let progressToken: String
    let compatible: CompatibilitySource?
    let chapters: [Chapter]
    let nextItemID: String?
    let subtitles: [ExternalSubtitle]
    let markers: [PlaybackMarker]
    let autoSkip: Set<String>
}

struct CompatibilitySource: Sendable {
    let url: URL
    let mode: String
    let reason: String
    let progressToken: String
    let timeline: MediaTimeline
}

struct ExternalSubtitle: Identifiable, Sendable {
    let label: String
    let language: String
    let url: URL
    let isDefault: Bool
    var id: String { url.absoluteString }
}

struct PlaybackMarker: Identifiable, Sendable {
    let type: String
    let label: String
    let start: Double
    let end: Double
    var id: String { "\(type):\(start)" }
}

struct ProgressSyncResult: Sendable {
    let progress: WatchProgress
    let conflict: Bool
}

struct Bookmark: Identifiable, Hashable, Sendable {
    let id: String
    let title: String
    let position: BookmarkPosition
}

enum BookmarkPosition: Hashable, Sendable {
    case playback(seconds: Double)
    case reading(page: Int, offset: Double)
}

struct PlaybackPreferences: Sendable, Equatable {
    var rate: Double = 1
    var audioLanguage = "auto"
    var subtitleLanguage = "auto"
    var audioTrack = ""
    var subtitleTrack = ""
    var nightMode = false
    var dialogueBoost = false
    var volumeBoost: Double = 1
}

struct ItemPlaybackPreferences: Sendable {
    let playback: PlaybackPreferences
    let overridden: Bool
}

enum ReaderTheme: String, CaseIterable, Sendable {
    case auto, light, dark, sepia
}

struct MediaPreferences: Sendable, Equatable {
    var playback = PlaybackPreferences()
    var autoDownloadNext = 0
    var removeWatched = false
    var downloadLimitGiB = 20
    var wifiOnly = true
    var readerFontSize = 20
    var readerTheme = ReaderTheme.auto
}

enum DownloadQuality: String, CaseIterable, Identifiable, Sendable {
    case original, compatible, fullHD = "1080p", hd = "720p", audio
    var id: String { rawValue }
    var title: String {
        switch self {
        case .audio: "Compatible audio"
        case .original: "Original"
        case .compatible: "Compatible"
        case .fullHD: "1080p"
        case .hd: "720p"
        }
    }
}

struct DownloadTrackSelection: Sendable {
    let audio: [Int]
    let subtitles: [Int]
}

struct DownloadTrackOptions: Sendable {
    struct Track: Identifiable, Sendable {
        let index: Int
        let label: String
        var id: Int { index }
    }
    let audio: [Track]
    let subtitles: [Track]
}

struct DownloadIdentity: Sendable {
    let serverID: String
    let profileID: String
}

struct PreparedDownload: Sendable {
    enum State: String, Sendable { case preparing, ready, failed }
    let id: String
    let itemID: String
    let quality: DownloadQuality
    let state: State
    let size: Int64
    let sha256: String
    let error: String?
}

struct OfflineManifest: Codable, Equatable, Sendable {
    let version: Int
    let id: String
    let size: Int64
    let sha256: String
    let chunkSize: Int64
    let chunks: [String]
}

struct OfflineDownload: Identifiable, Sendable {
    enum State: String, Sendable {
        case preparing, queued, downloading, waiting, paused, verifying, ready, failed
    }
    let id: String
    let item: MediaItem
    let quality: DownloadQuality
    let state: State
    let receivedBytes: Int64
    let totalBytes: Int64
    let message: String?
}

struct ReaderBook: Identifiable, Sendable {
    enum Kind: String, Sendable { case epub, pdf, comic }
    struct Page: Identifiable, Sendable {
        let number: Int
        let title: String
        let resource: URL
        var id: Int { number }
    }
    let id: String
    let title: String
    let kind: Kind
    let pages: [Page]
}

struct ReaderPosition: Equatable, Sendable {
    let page: Int
    let total: Int
    let offset: Double
}

struct CastDevice: Identifiable, Sendable {
    let id: String
    let name: String
    let receiverProtocol: CastProtocol
}

enum CastProtocol: String, Sendable { case googleCast = "google-cast", dlna }

struct CastStart: Sendable {
    let receiverProtocol: CastProtocol
    let deviceID: String?
    let position: Double
    let playbackToken: String?
}

struct CastSession: Sendable {
    struct Track: Identifiable, Sendable {
        let id: Int
        let url: URL
        let label: String
        let language: String
        let isDefault: Bool
    }
    let id: String
    let url: URL
    let contentType: String
    let title: String
    let position: Double
    let duration: Double
    let expiresAt: Date
    let receiverProtocol: CastProtocol
    let tracks: [Track]
    let deviceID: String?
    let deviceName: String?
}

struct CastStatus: Sendable {
    enum State: String, Sendable { case playing, paused, buffering, stopped }
    let state: State
    let position: Double
    let duration: Double
}

enum CastCommand: Sendable {
    case play, pause, stop, seek(Double)
}
