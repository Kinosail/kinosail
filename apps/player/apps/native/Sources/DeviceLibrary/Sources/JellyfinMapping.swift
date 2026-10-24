import Foundation

enum JellyfinMapping {
    static func item(_ raw: [String: Any], source: String) -> DeviceItem? {
        guard let id = SourceHTTP.identifier(raw["Id"]) else { return nil }
        let kind: DeviceItem.Kind
        switch raw["Type"] as? String {
        case "Movie": kind = .movie
        case "Series": kind = .series
        case "Episode": kind = .episode
        default: return nil
        }
        let title = SourceHTTP.text(raw["Name"])
        guard !title.isEmpty else { return nil }
        var item = DeviceItem(kind: kind, ref: SourceRef(source: source, item: id), title: title,
                              sortTitle: DeviceItem.sortable(SourceHTTP.text(raw["SortName"]).isEmpty ? title : SourceHTTP.text(raw["SortName"])))
        if let year = raw["ProductionYear"] as? Int, (1800...3000).contains(year) { item.year = String(year) }
        item.plot = SourceHTTP.text(raw["Overview"], max: 10_000)
        item.tagline = SourceHTTP.text((raw["Taglines"] as? [Any])?.first)
        item.rating = SourceHTTP.text(raw["OfficialRating"], max: 32)
        item.genres = ((raw["Genres"] as? [Any]) ?? []).prefix(4).map { SourceHTTP.text($0, max: 64) }.filter { !$0.isEmpty }.joined(separator: ", ")
        item.studio = SourceHTTP.text(((raw["Studios"] as? [[String: Any]])?.first)?["Name"])
        item.added = SourceHTTP.date(raw["DateCreated"])
        if let ticks = raw["RunTimeTicks"] as? Double ?? (raw["RunTimeTicks"] as? Int).map(Double.init), ticks > 0 { item.duration = ticks / 10_000_000 }
        item.container = String(SourceHTTP.text(raw["Container"], max: 64).split(separator: ",").first ?? "").lowercased()
        item.width = raw["Width"] as? Int ?? 0
        item.providers = providers(raw["ProviderIds"])
        let images = raw["ImageTags"] as? [String: Any] ?? [:]
        item.hasPrimary = images["Primary"] != nil
        item.hasBackdrop = !((raw["BackdropImageTags"] as? [Any]) ?? []).isEmpty
        if kind == .episode {
            item.seriesTitle = SourceHTTP.text(raw["SeriesName"])
            item.seriesRef = SourceHTTP.identifier(raw["SeriesId"]).map { SourceRef(source: source, item: $0) }
            item.season = raw["ParentIndexNumber"] as? Int ?? 0
            item.episode = raw["IndexNumber"] as? Int ?? 0
            item.seriesHasPrimary = raw["SeriesPrimaryImageTag"] != nil
            item.seriesHasBackdrop = !((raw["ParentBackdropImageTags"] as? [Any]) ?? []).isEmpty
            if item.seriesTitle.isEmpty || item.seriesRef == nil { return nil }
        }
        if let data = raw["UserData"] as? [String: Any] {
            let ticks = data["PlaybackPositionTicks"] as? Double ?? (data["PlaybackPositionTicks"] as? Int).map(Double.init) ?? 0
            item.state = DeviceUserState(seconds: max(0, ticks / 10_000_000), watched: data["Played"] as? Bool ?? false,
                                         favorite: data["IsFavorite"] as? Bool ?? false, lastPlayed: SourceHTTP.date(data["LastPlayedDate"]))
        }
        let people = (raw["People"] as? [[String: Any]]) ?? []
        item.director = SourceHTTP.text(people.first { ($0["Type"] as? String) == "Director" }?["Name"])
        item.cast = people.filter { ["Actor", "GuestStar"].contains($0["Type"] as? String ?? "") }.prefix(24).compactMap { person in
            let name = SourceHTTP.text(person["Name"], max: 256)
            guard !name.isEmpty else { return nil }
            let image = person["PrimaryImageTag"] != nil ? SourceHTTP.identifier(person["Id"]).map { SourceRef(source: source, item: $0) } : nil
            return DevicePerson(name: name, role: SourceHTTP.text(person["Role"], max: 256), image: image)
        }
        if let sources = raw["MediaSources"] as? [[String: Any]], let first = sources.first {
            item.size = (first["Size"] as? Int64) ?? Int64(first["Size"] as? Int ?? 0)
            if item.container.isEmpty { item.container = SourceHTTP.text(first["Container"], max: 32).lowercased() }
        }
        return item
    }

    static func providers(_ raw: Any?) -> [String: String] {
        var result: [String: String] = [:]
        for (key, value) in raw as? [String: Any] ?? [:] {
            let name = key.lowercased()
            guard ["tmdb", "imdb", "tvdb"].contains(name) else { continue }
            let text = SourceHTTP.text(value, max: 32)
            if !text.isEmpty, text.range(of: "^[A-Za-z0-9]+$", options: .regularExpression) != nil { result[name] = text }
        }
        return result
    }
}
