import Foundation

enum JellyfinPlayback {
    static func profile(_ capabilities: DeviceCapabilities) -> [String: Any] {
        let video = ["hevc", "h264", "av1"].filter { capabilities.videoCodecs.contains($0) }.joined(separator: ",")
        let direct = ["aac", "ac3", "eac3", "mp3"].filter { capabilities.audioCodecs.contains($0) || $0 == "aac" }.joined(separator: ",")
        let audio = capabilities.convertedAudioCodecs.joined(separator: ",")
        return [
            "MaxStreamingBitrate": 200_000_000,
            "MaxStaticBitrate": 200_000_000,
            "DirectPlayProfiles": [["Container": "mp4,m4v,mov", "Type": "Video", "VideoCodec": video, "AudioCodec": direct + ",alac,flac"],
                                   ["Container": "mp3,aac,m4a,m4b,flac,alac,wav", "Type": "Audio"]],
            "TranscodingProfiles": [["Container": "mp4", "Type": "Video", "Protocol": "hls", "VideoCodec": video, "AudioCodec": audio,
                                     "Context": "Streaming", "MaxAudioChannels": String(capabilities.maxAudioChannels),
                                     "MinSegments": 1, "BreakOnNonKeyFrames": true],
                                    ["Container": "aac", "Type": "Audio", "Protocol": "hls", "AudioCodec": "aac", "Context": "Streaming"]],
            "SubtitleProfiles": [["Format": "vtt", "Method": "External"], ["Format": "pgssub", "Method": "Encode"],
                                 ["Format": "dvdsub", "Method": "Encode"]],
            "CodecProfiles": [], "ContainerProfiles": [], "ResponseProfiles": []
        ]
    }

    static func info(client: JellyfinClient, id rawID: String, capabilities: DeviceCapabilities, start: Double) async throws -> DevicePlaybackInfo {
        guard let id = SourceHTTP.identifier(rawID) else { throw DeviceFailure.notFound }
        let account = client.account
        let ticks = String(Int64(max(0, start) * 10_000_000))
        let body: [String: Any] = ["DeviceProfile": profile(capabilities), "UserId": account.userID, "EnableDirectPlay": true,
                                   "EnableDirectStream": true, "EnableTranscoding": true, "SubtitleStreamIndex": -1,
                                   "AutoOpenLiveStream": true, "StartTimeTicks": Int64(ticks) ?? 0]
        let value = try await SourceHTTP.json(client.request("/Items/\(id)/PlaybackInfo",
            ["UserId": account.userID, "IsPlayback": "true", "AutoOpenLiveStream": "true", "StartTimeTicks": ticks], method: "POST", body: body))
        guard let source = (value["MediaSources"] as? [[String: Any]])?.first,
              let sourceID = SourceHTTP.identifier(source["Id"]) else { throw DeviceFailure.notFound }
        let session = SourceHTTP.identifier(value["PlaySessionId"]) ?? UUID().uuidString.replacingOccurrences(of: "-", with: "")
        let streams = source["MediaStreams"] as? [[String: Any]] ?? []
        let video = streams.first { ($0["Type"] as? String) == "Video" } ?? [:]
        func stream(_ raw: [String: Any]) -> DevicePlaybackInfo.Stream? {
            guard let index = raw["Index"] as? Int else { return nil }
            let codec = SourceHTTP.text(raw["Codec"], max: 32).lowercased()
            let external: URL?
            if (raw["Type"] as? String) == "Subtitle", !["pgssub", "dvdsub", "dvbsub", "pgs", "vobsub"].contains(codec) {
                external = try? SourceHTTP.url(account.address, "/Videos/\(id)/\(sourceID)/Subtitles/\(index)/0/Stream.vtt")
            } else { external = nil }
            return DevicePlaybackInfo.Stream(index: index, codec: codec, language: SourceHTTP.text(raw["Language"], max: 16).lowercased(),
                                             title: SourceHTTP.text(raw["DisplayTitle"] ?? raw["Title"], max: 256),
                                             isDefault: raw["IsDefault"] as? Bool ?? false, isForced: raw["IsForced"] as? Bool ?? false,
                                             isExternal: raw["IsExternal"] as? Bool ?? false, channels: raw["Channels"] as? Int ?? 0,
                                             deliveryURL: external)
        }
        let audio = streams.filter { ($0["Type"] as? String) == "Audio" }.compactMap(stream)
        let subtitles = streams.filter { ($0["Type"] as? String) == "Subtitle" }.compactMap(stream)
        let duration = ((source["RunTimeTicks"] as? Double) ?? (source["RunTimeTicks"] as? Int).map(Double.init) ?? 0) / 10_000_000
        let container = SourceHTTP.text(source["Container"], max: 32).lowercased()
        let direct = try client.request("/Videos/\(id)/stream", ["static": "true", "MediaSourceId": sourceID, "PlaySessionId": session,
                                                                 "DeviceId": SourceHTTP.installID])
        var transcode: URLRequest?
        if let path = source["TranscodingUrl"] as? String, path.hasPrefix("/"), path.utf8.count <= 8192,
           let url = URL(string: account.address.absoluteString + path) {
            var request = try JellyfinClient.request(url, token: account.token)
            request.url = url
            transcode = request
        }
        let directPlay = source["SupportsDirectPlay"] as? Bool ?? false
        let chapters = await chaptersAndMarkers(client: client, id: id)
        return DevicePlaybackInfo(ref: SourceRef(source: account.id, item: id), mediaSourceID: sourceID, container: container,
                                  bitrate: source["Bitrate"] as? Int ?? 0, duration: duration,
                                  size: (source["Size"] as? Int64) ?? Int64(source["Size"] as? Int ?? 0),
                                  videoCodec: SourceHTTP.text(video["Codec"], max: 32).lowercased(),
                                  videoProfile: SourceHTTP.text(video["Profile"], max: 64),
                                  videoRange: SourceHTTP.text(video["VideoRangeType"] ?? video["VideoRange"], max: 32),
                                  width: video["Width"] as? Int ?? 0, height: video["Height"] as? Int ?? 0,
                                  audio: audio, subtitles: subtitles, chapters: chapters.chapters, markers: chapters.markers,
                                  direct: direct, transcode: transcode, directPlayable: directPlay, session: session)
    }

    private static func chaptersAndMarkers(client: JellyfinClient, id: String) async -> (chapters: [(title: String, start: Double)], markers: [(type: String, start: Double, end: Double)]) {
        var chapters: [(title: String, start: Double)] = []
        var markers: [(type: String, start: Double, end: Double)] = []
        if let value = try? await SourceHTTP.json(client.request("/Users/\(client.account.userID)/Items/\(id)", ["Fields": "Chapters"])) {
            for raw in (value["Chapters"] as? [[String: Any]] ?? []).prefix(256) {
                let ticks = (raw["StartPositionTicks"] as? Double) ?? (raw["StartPositionTicks"] as? Int).map(Double.init) ?? -1
                guard ticks >= 0 else { continue }
                let type = raw["MarkerType"] as? String ?? ""
                if type == "IntroStart" { markers.append(("intro", ticks / 10_000_000, ticks / 10_000_000)) }
                else if type == "IntroEnd", let index = markers.lastIndex(where: { $0.type == "intro" }) { markers[index].end = ticks / 10_000_000 }
                else if type == "CreditsStart" { markers.append(("credits", ticks / 10_000_000, .infinity)) }
                else { chapters.append((SourceHTTP.text(raw["Name"], max: 256), ticks / 10_000_000)) }
            }
        }
        if client.account.kind == .jellyfin, let value = try? await SourceHTTP.json(client.request("/MediaSegments/\(id)")) {
            for raw in (value["Items"] as? [[String: Any]] ?? []).prefix(64) {
                let start = ((raw["StartTicks"] as? Double) ?? (raw["StartTicks"] as? Int).map(Double.init) ?? -1) / 10_000_000
                let end = ((raw["EndTicks"] as? Double) ?? (raw["EndTicks"] as? Int).map(Double.init) ?? -1) / 10_000_000
                let type = switch raw["Type"] as? String {
                case "Intro": "intro"
                case "Outro": "credits"
                case "Recap": "recap"
                case "Commercial": "commercial"
                default: ""
                }
                if !type.isEmpty, start >= 0, end > start { markers.append((type, start, end)) }
            }
        }
        return (chapters.sorted { $0.start < $1.start }, markers.filter { $0.end > $0.start })
    }
}
