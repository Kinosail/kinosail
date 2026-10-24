import Foundation

enum PlexPlayback {
    static func info(client: PlexClient, id: String, capabilities: DeviceCapabilities, start: Double) async throws -> DevicePlaybackInfo {
        let (raw, item) = try await client.metadata(id, extras: ["includeChapters": "1", "includeMarkers": "1"])
        guard let media = (raw["Media"] as? [[String: Any]])?.first, let part = (media["Part"] as? [[String: Any]])?.first,
              let key = part["key"] as? String, key.utf8.count <= 512,
              key.range(of: "^/library/parts/[0-9]+/[0-9]+/file\\.[a-z0-9]{1,8}$", options: .regularExpression) != nil,
              let partID = SourceHTTP.identifier(part["id"]) else { throw DeviceFailure.notFound }
        let base = try await PlexRoutes.shared.resolve(client.account)
        let session = UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased()
        let container = SourceHTTP.text(media["container"], max: 32).lowercased()
        let videoCodec = SourceHTTP.text(media["videoCodec"], max: 32).lowercased()
        let audioCodec = SourceHTTP.text(media["audioCodec"], max: 32).lowercased()
        let streams = (part["Stream"] as? [[String: Any]]) ?? []
        let video = streams.first { $0["streamType"] as? Int == 1 } ?? [:]
        let audioAllowed = capabilities.audioCodecs.union(["aac", "alac", "flac", "mp3"])
        let direct = ["mp4", "m4v", "mov"].contains(container) && capabilities.videoCodecs.contains(videoCodec)
            && (audioCodec.isEmpty || audioAllowed.contains(audioCodec))
        let directRequest = try client.request(key, base: base)
        var copy = ["h264"]
        if capabilities.videoCodecs.contains("hevc") { copy.append("hevc") }
        let audio = ["aac", "ac3", "eac3"].filter { $0 == "aac" || capabilities.audioCodecs.contains($0) }.joined(separator: ",")
        var transcode = try client.request("/video/:/transcode/universal/start.m3u8", [
            "path": "/library/metadata/\(item.ref.item)", "protocol": "hls", "directPlay": "0", "directStream": "1",
            "directStreamAudio": "1", "mediaIndex": "0", "partIndex": "0", "fastSeek": "1", "offset": "0",
            "session": session, "subtitles": "none", "maxAudioChannels": String(capabilities.maxAudioChannels),
            "X-Plex-Client-Profile-Extra": "append-transcode-target-codec(type=videoProfile&context=streaming&protocol=hls&videoCodec=\(copy.joined(separator: ","))&audioCodec=\(audio))"
        ], base: base)
        transcode.setValue(session, forHTTPHeaderField: "X-Plex-Session-Identifier")
        func stream(_ raw: [String: Any]) -> DevicePlaybackInfo.Stream? {
            guard let index = raw["id"] as? Int else { return nil }
            let codec = SourceHTTP.text(raw["codec"], max: 32).lowercased()
            var delivery: URL?
            if raw["streamType"] as? Int == 3, ["srt", "subrip", "vtt", "webvtt"].contains(codec),
               let path = raw["key"] as? String, path.range(of: "^/library/streams/[0-9]+$", options: .regularExpression) != nil {
                delivery = try? SourceHTTP.url(base, path, ["encoding": "utf-8"])
            }
            let language = SourceHTTP.text(raw["languageTag"] ?? raw["languageCode"], max: 16).lowercased()
            return DevicePlaybackInfo.Stream(index: index, codec: codec, language: language,
                                             title: SourceHTTP.text(raw["extendedDisplayTitle"] ?? raw["displayTitle"], max: 256),
                                             isDefault: raw["default"] as? Bool ?? raw["selected"] as? Bool ?? false,
                                             isForced: raw["forced"] as? Bool ?? false, isExternal: raw["key"] != nil,
                                             channels: raw["channels"] as? Int ?? 0, deliveryURL: delivery)
        }
        let duration = (PlexMapping.number(media["duration"]) ?? PlexMapping.number(raw["duration"]) ?? 0) / 1000
        return DevicePlaybackInfo(ref: item.ref, mediaSourceID: partID, container: container,
                                  bitrate: Int(PlexMapping.number(media["bitrate"]) ?? 0) * 1000, duration: duration,
                                  size: Int64(PlexMapping.number(part["size"]) ?? 0), videoCodec: videoCodec,
                                  videoProfile: SourceHTTP.text(video["profile"], max: 64),
                                  videoRange: SourceHTTP.text(video["DOVIPresent"] as? Bool == true ? "DOVI" : video["colorTrc"], max: 32),
                                  width: media["width"] as? Int ?? 0, height: media["height"] as? Int ?? 0,
                                  audio: streams.filter { $0["streamType"] as? Int == 2 }.compactMap(stream),
                                  subtitles: streams.filter { $0["streamType"] as? Int == 3 }.compactMap(stream),
                                  chapters: chapters(raw), markers: markers(raw),
                                  direct: directRequest, transcode: transcode, directPlayable: direct, session: session)
    }

    static func chapters(_ raw: [String: Any]) -> [(title: String, start: Double)] {
        ((raw["Chapter"] as? [[String: Any]]) ?? []).prefix(256).compactMap { chapter in
            guard let start = PlexMapping.number(chapter["startTimeOffset"]), start >= 0 else { return nil }
            return (SourceHTTP.text(chapter["tag"], max: 256), start / 1000)
        }.sorted { $0.start < $1.start }
    }

    static func markers(_ raw: [String: Any]) -> [(type: String, start: Double, end: Double)] {
        ((raw["Marker"] as? [[String: Any]]) ?? []).prefix(64).compactMap { marker in
            guard let type = marker["type"] as? String, ["intro", "credits"].contains(type),
                  let start = PlexMapping.number(marker["startTimeOffset"]), let end = PlexMapping.number(marker["endTimeOffset"]),
                  start >= 0, end > start else { return nil }
            return (type, start / 1000, end / 1000)
        }
    }
}
