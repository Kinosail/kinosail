import Foundation

enum DevicePlayback {
    static func choose(_ infos: [(info: DevicePlaybackInfo, local: Bool)]) -> DevicePlaybackInfo? {
        infos.max { left, right in
            let a = (left.info.directPlayable ? 1 : 0, left.info.height, left.local ? 1 : 0)
            let b = (right.info.directPlayable ? 1 : 0, right.info.height, right.local ? 1 : 0)
            return a < b
        }?.info
    }

    static func contentType(_ container: String) -> String {
        switch container {
        case "mp4", "m4v": "video/mp4"
        case "mov": "video/quicktime"
        case "mkv": "video/x-matroska"
        case "webm": "video/webm"
        case "avi": "video/x-msvideo"
        case "ts", "m2ts", "mpegts": "video/mp2t"
        default: "application/octet-stream"
        }
    }

    static func json(item: DeviceItem, info: DevicePlaybackInfo, start: Double, next: String?, compatiblePath: String?) -> JSONValue {
        let duration = info.duration > 0 ? info.duration : item.duration
        let total = max(duration, 0.001)
        let container = info.container.split(separator: ",").first.map(String.init) ?? info.container
        var media: [String: JSONValue] = ["kind": .string("video"), "container": .string(String(container.prefix(32))),
                                          "duration": .number(total), "seekable": .bool(true)]
        if info.bitrate > 0 { media["bitrate"] = .number(Double(info.bitrate)) }
        var value: [String: JSONValue] = [
            "policy": .string("automatic"), "media": .object(media), "duration": .number(total),
            "start": .number(start > 0 && start < total ? start : 0), "progressToken": .string(""),
            "plan": .object(["allowed": .bool(info.directPlayable), "mode": .string(info.directPlayable ? "direct" : "denied"),
                             "reason": .string(info.directPlayable ? "original" : "format")]),
            "directAllowed": .bool(info.directPlayable)
        ]
        if info.directPlayable {
            value["direct"] = .string("/media/\(item.id)")
            value["directType"] = .string(contentType(container))
        }
        if let compatiblePath {
            value["compatible"] = .string(compatiblePath)
            value["compatiblePlan"] = .object(["allowed": .bool(true), "mode": .string("transcode"), "reason": .string("source-conversion")])
            value["compatibleDuration"] = .number(total)
        }
        if let next { value["next"] = .string(next) }
        var priorStart = -1.0
        value["chapters"] = .array(info.chapters.enumerated().compactMap { index, chapter in
            guard chapter.start >= priorStart, chapter.start < total else { return nil }
            priorStart = chapter.start
            let end = index + 1 < info.chapters.count ? min(total, max(chapter.start, info.chapters[index + 1].start)) : total
            return .object(["title": .string(String(chapter.title.prefix(512))), "start": .number(chapter.start), "end": .number(end)])
        })
        var seen = Set<String>()
        value["markers"] = .array(info.markers.compactMap { marker in
            let end = min(marker.end, total)
            guard marker.start >= 0, end > marker.start, seen.insert("\(marker.type):\(marker.start)").inserted else { return nil }
            return .object(["type": .string(marker.type), "start": .number(marker.start), "end": .number(end)])
        })
        value["subtitles"] = .array(info.subtitles.filter { $0.deliveryURL != nil }.prefix(64).map { stream in
            var track: [String: JSONValue] = ["label": .string(label(stream)), "source": .string("/subtitle/\(item.id)/\(stream.index).vtt"),
                                              "default": .bool(stream.isDefault)]
            if !stream.language.isEmpty, stream.language.range(of: "^[a-z]{2,3}(-[a-z0-9]{2,8})?$", options: .regularExpression) != nil {
                track["language"] = .string(stream.language)
            }
            if stream.isForced { track["forced"] = .bool(true) }
            return .object(track)
        })
        return .object(value)
    }

    static func label(_ stream: DevicePlaybackInfo.Stream) -> String {
        let text = stream.title.isEmpty ? (stream.language.isEmpty ? "Subtitles \(stream.index)" : Locale.current.localizedString(forLanguageCode: stream.language) ?? stream.language) : stream.title
        return String(text.prefix(512))
    }
}
