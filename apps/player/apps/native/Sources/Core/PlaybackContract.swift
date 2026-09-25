import Foundation

extension PlaybackSource {
    init(_ raw: JSONValue, itemID: String, server: ServerAddress) throws {
        let value = try raw.object(allowing: ["media", "plan", "compatiblePlan", "compatibleLabel", "compatibleDescription", "qualities",
                                            "directAllowed", "direct", "compatibleDuration", "compatibleProgressToken", "compatible", "download", "directType",
                                            "summary", "duration", "start", "audio", "chapters", "markers", "autoSkip", "subtitles", "next", "downloadNext",
                                            "trickplay", "progressToken", "replayGain"])
        let plan = try Self.plan(value.required("plan"))
        let media = try value.required("media").object(allowing: ["kind", "fileVersion", "container", "bitrate", "duration", "seekable", "video", "audio", "subtitles"])
        let reportedDuration = try value.number("duration")
        let mediaDuration = try media.number("duration", fallback: reportedDuration)
        let primary: MediaTimeline
        if try plan.text("markerMode", max: 32) == "server" {
            primary = try MediaTimeline(plan.required("timeline"))
            guard abs(primary.sourceDuration - mediaDuration) < 0.01, abs(primary.duration - reportedDuration) < 0.01 else { throw ClientError.invalidResponse }
        } else { primary = try MediaTimeline(sourceDuration: reportedDuration, duration: reportedDuration) }
        duration = mediaDuration
        let requestedStart = try value.number("start")
        // Saved progress can outlive a replaced or shortened media file.
        start = primary.sourceTime(requestedStart < primary.duration ? requestedStart : 0)
        progressToken = try value.text("progressToken", max: 8192)
        let preview = try value.text("trickplay", max: 16_384)
        if preview.isEmpty { trickplay = nil }
        else {
            let prefix = "/trickplay/\(itemID)/{second}"
            guard preview.hasPrefix(prefix),
                  let url = try? server.mediaURL(preview.replacingOccurrences(of: "{second}", with: "0")),
                  url.path == "/trickplay/\(itemID)/0" else { throw ClientError.invalidResponse }
            let query = URLComponents(url: url, resolvingAgainstBaseURL: false)?.queryItems
            guard preview == prefix && query == nil ||
                  query?.count == 1 && query?[0].name == "playbackToken" && query?[0].value == progressToken && !progressToken.isEmpty
            else { throw ClientError.invalidResponse }
            trickplay = preview
        }
        let path = try value.text("direct", max: 16_384)
        let allowed = try value.flag("directAllowed")
        if allowed {
            let url = try server.mediaURL(path)
            guard url.path == "/media/\(itemID)", url.query == nil else { throw ClientError.invalidResponse }
            direct = url
        } else {
            guard path.isEmpty else { throw ClientError.invalidResponse }
            direct = nil
        }
        contentType = try value.text("directType", max: 128)
        let compatiblePath = try value.text("compatible", max: 16_384)
        if !compatiblePath.isEmpty {
            let fallback = try Self.plan(value.required("compatiblePlan"))
            guard try fallback.flag("allowed"), try fallback.text("mode") != "denied" else { throw ClientError.invalidResponse }
            let url = try server.mediaURL(compatiblePath)
            guard url.path.hasPrefix("/hls/\(itemID)/"), url.path.hasSuffix(".m3u8") else { throw ClientError.invalidResponse }
            let presentedDuration = try value.number("compatibleDuration", fallback: mediaDuration)
            let timeline: MediaTimeline
            if try fallback.text("markerMode", max: 32) == "server" { timeline = try MediaTimeline(fallback.required("timeline")) }
            else { timeline = try MediaTimeline(sourceDuration: mediaDuration, duration: presentedDuration) }
            guard abs(timeline.duration - presentedDuration) < 0.01, abs(timeline.sourceDuration - mediaDuration) < 0.01 else { throw ClientError.invalidResponse }
            let token = try value.text("compatibleProgressToken", max: 8192)
            guard timeline.omitted.isEmpty || !token.isEmpty else { throw ClientError.invalidResponse }
            compatible = try CompatibilitySource(url: url, mode: fallback.text("mode"), reason: fallback.text("reason", max: 128, required: true),
                                                 progressToken: token, timeline: timeline)
        } else { compatible = nil }
        guard direct != nil || compatible != nil else { throw ClientError.http(403) }
        var priorStart: Double = -1
        chapters = try value.list("chapters", max: 1024).enumerated().map { index, raw in
            let chapter = try raw.object(allowing: ["index", "title", "start", "end"])
            if chapter["index"] != nil {
                guard try chapter.requiredNumber("index", max: 1023, integer: true) == Double(index) else { throw ClientError.invalidResponse }
            }
            let begin = try chapter.number("start", max: primary.duration)
            let end = try chapter.requiredNumber("end", max: primary.duration)
            guard begin >= priorStart, end >= begin else { throw ClientError.invalidResponse }
            priorStart = begin
            return try Chapter(title: chapter.text("title", max: 512), start: primary.sourceTime(begin), end: primary.sourceTime(end))
        }
        let next = try value.text("next", max: 128)
        nextItemID = next.isEmpty ? nil : try Input.id(next)
        subtitles = try Input.unique(value.list("subtitles", max: 256).map { raw in
            let track = try raw.object(allowing: ["label", "source", "default", "language", "role", "kind", "forced", "embedded"])
            let url = try server.mediaURL(track.text("source", max: 2048, required: true))
            guard url.path.hasPrefix("/subtitle/\(itemID)/"), url.query == nil else { throw ClientError.invalidResponse }
            return try ExternalSubtitle(label: track.text("label", max: 512, required: true), language: track.text("language", max: 32),
                                        url: url, isDefault: track.flag("default"))
        })
        let types: Set<String> = ["intro", "recap", "commercial", "outro", "credits"]
        markers = try Input.unique(value.list("markers", max: 128).map { raw in
            let marker = try raw.object(allowing: ["type", "label", "start", "end", "source"])
            let type = try marker.text("type", max: 32)
            let begin = try marker.number("start", max: primary.duration)
            let end = try marker.requiredNumber("end", max: primary.duration)
            guard types.contains(type), end > begin else { throw ClientError.invalidResponse }
            return try PlaybackMarker(type: type, label: marker.text("label", max: 512), start: primary.sourceTime(begin), end: primary.sourceTime(end))
        })
        let skip = try value.list("autoSkip", max: 5).map { raw in
            guard case .string(let type) = raw, types.contains(type) else { throw ClientError.invalidResponse }
            return type
        }
        guard Set(skip).count == skip.count else { throw ClientError.invalidResponse }
        autoSkip = Set(skip)
    }

    private static func plan(_ raw: JSONValue) throws -> [String: JSONValue] {
        let value = try raw.object(allowing: ["allowed", "mode", "reason", "container", "videoCodec", "audioCodec", "subtitleMode", "colorMode", "audioIndex", "subtitleIndex",
                                            "subtitleSourceIndex", "subtitleText", "subtitleExternal", "subtitleExternalIndex", "maxBitrate", "width", "height", "adaptive", "qualities", "markerMode", "timeline"])
        let mode = try value.text("mode", max: 32, required: true)
        guard ["direct", "remux", "audio-transcode", "transcode", "denied"].contains(mode) else { throw ClientError.invalidResponse }
        let allowed = try value.flag("allowed")
        guard mode != "denied" || !allowed else { throw ClientError.invalidResponse }
        _ = try value.text("reason", max: 128, required: true)
        return value
    }
}

extension WatchProgress {
    func validated(required: Bool) throws -> WatchProgress {
        try Input.position(seconds)
        _ = try Input.text(session, max: 128, label: "playback session", empty: !required)
        guard (0...9_007_199_254_740_991).contains(revision), !required || revision > 0 else { throw ClientError.invalidInput("The playback revision is invalid.") }
        return self
    }
}
