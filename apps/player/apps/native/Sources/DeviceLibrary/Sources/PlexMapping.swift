import Foundation

enum PlexMapping {
    static func item(_ raw: [String: Any], source: String) -> DeviceItem? {
        guard let id = SourceHTTP.identifier(raw["ratingKey"]) else { return nil }
        let kind: DeviceItem.Kind
        switch raw["type"] as? String {
        case "movie": kind = .movie
        case "show": kind = .series
        case "episode": kind = .episode
        default: return nil
        }
        let title = SourceHTTP.text(raw["title"])
        guard !title.isEmpty else { return nil }
        let sort = SourceHTTP.text(raw["titleSort"])
        var item = DeviceItem(kind: kind, ref: SourceRef(source: source, item: id), title: title,
                              sortTitle: DeviceItem.sortable(sort.isEmpty ? title : sort))
        if let year = raw["year"] as? Int, (1800...3000).contains(year) { item.year = String(year) }
        item.plot = SourceHTTP.text(raw["summary"], max: 10_000)
        item.tagline = SourceHTTP.text(raw["tagline"])
        item.rating = SourceHTTP.text(raw["contentRating"], max: 32)
        item.genres = tags(raw["Genre"]).prefix(4).joined(separator: ", ")
        item.studio = SourceHTTP.text(raw["studio"])
        item.director = tags(raw["Director"]).first ?? ""
        item.added = date(raw["addedAt"])
        if let duration = number(raw["duration"]), duration > 0 { item.duration = duration / 1000 }
        item.providers = providers(raw["Guid"])
        item.hasPrimary = raw["thumb"] is String
        item.hasBackdrop = raw["art"] is String
        if let media = (raw["Media"] as? [[String: Any]])?.first {
            item.container = String(SourceHTTP.text(media["container"], max: 64).split(separator: ",").first ?? "").lowercased()
            item.width = media["width"] as? Int ?? 0
            if let part = (media["Part"] as? [[String: Any]])?.first { item.size = Int64(number(part["size"]) ?? 0) }
        }
        if kind == .episode {
            item.seriesTitle = SourceHTTP.text(raw["grandparentTitle"])
            item.seriesRef = SourceHTTP.identifier(raw["grandparentRatingKey"]).map { SourceRef(source: source, item: $0) }
            item.season = raw["parentIndex"] as? Int ?? 0
            item.episode = raw["index"] as? Int ?? 0
            item.seriesHasPrimary = raw["grandparentThumb"] is String
            item.seriesHasBackdrop = raw["grandparentArt"] is String
            if item.seriesTitle.isEmpty || item.seriesRef == nil { return nil }
        }
        let offset = (number(raw["viewOffset"]) ?? 0) / 1000
        item.state = DeviceUserState(seconds: max(0, offset), watched: (number(raw["viewCount"]) ?? 0) > 0 && offset == 0,
                                     favorite: false, lastPlayed: date(raw["lastViewedAt"]))
        item.cast = ((raw["Role"] as? [[String: Any]]) ?? []).prefix(24).compactMap { person in
            let name = SourceHTTP.text(person["tag"], max: 256)
            guard !name.isEmpty else { return nil }
            return DevicePerson(name: name, role: SourceHTTP.text(person["role"], max: 256), image: portrait(person["thumb"]).map { SourceRef(source: source, item: $0) })
        }
        return item
    }

    static func tags(_ raw: Any?) -> [String] {
        ((raw as? [[String: Any]]) ?? []).prefix(16).map { SourceHTTP.text($0["tag"], max: 64) }.filter { !$0.isEmpty }
    }

    static func providers(_ raw: Any?) -> [String: String] {
        var result: [String: String] = [:]
        for entry in ((raw as? [[String: Any]]) ?? []).prefix(16) {
            guard let value = entry["id"] as? String, value.utf8.count <= 64 else { continue }
            let parts = value.components(separatedBy: "://")
            guard parts.count == 2, ["tmdb", "imdb", "tvdb"].contains(parts[0]),
                  parts[1].range(of: "^[A-Za-z0-9]{1,32}$", options: .regularExpression) != nil else { continue }
            result[parts[0]] = parts[1]
        }
        return result
    }

    static func portrait(_ raw: Any?) -> String? {
        guard let text = raw as? String, let url = URL(string: text), url.scheme == "https", url.host == "metadata-static.plex.tv",
              url.path.range(of: "^/[0-9a-z]/people/[0-9a-f]{32}\\.(jpg|png)$", options: .regularExpression) != nil else { return nil }
        return String(url.path.dropFirst())
    }

    static func number(_ raw: Any?) -> Double? {
        if let value = raw as? Double { return value }
        if let value = raw as? Int { return Double(value) }
        if let value = raw as? NSNumber { return value.doubleValue }
        return nil
    }

    static func date(_ raw: Any?) -> Date? {
        guard let seconds = number(raw), seconds > 0, seconds < 32_503_680_000 else { return nil }
        return Date(timeIntervalSince1970: seconds)
    }
}
