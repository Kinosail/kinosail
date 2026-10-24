import Foundation

enum DeviceProjection {
    static func item(_ item: DeviceItem, mark: ProgressMark) -> JSONValue {
        var value: [String: JSONValue] = [
            "id": .string(item.id), "title": .string(String(item.title.prefix(512))),
            "progress": progress(item.state, mark: mark)
        ]
        func set(_ key: String, _ text: String, max: Int = 512) { if !text.isEmpty { value[key] = .string(String(text.prefix(max))) } }
        set("sortTitle", item.sortTitle)
        set("year", item.year, max: 16)
        set("plot", item.plot, max: 10_000)
        set("tagline", item.tagline)
        set("rating", item.rating, max: 32)
        set("genres", item.genres)
        set("studio", item.studio)
        set("director", item.director)
        if let added = item.added { value["added"] = .string(ISO8601DateFormatter().string(from: added)) }
        switch item.kind {
        case .movie:
            value["kind"] = .string("video")
            if item.hasPrimary { value["artwork"] = .string("/art/\(item.id)") }
            if item.hasBackdrop { value["backdrop"] = .string("/backdrop/\(item.id)") }
            value["stream"] = .string("/media/\(item.id)")
        case .series:
            value["kind"] = .string("show")
            value["showId"] = .string(item.id)
            if item.hasPrimary { value["artwork"] = .string("/art/\(item.id)") }
            if item.hasBackdrop { value["backdrop"] = .string("/backdrop/\(item.id)") }
        case .episode:
            value["kind"] = .string("video")
            set("show", item.seriesTitle, max: 256)
            if let key = item.showKey { value["showId"] = .string(DeviceItem.identifier(key)) }
            value["season"] = .number(Double(min(100_000, max(0, item.season))))
            value["episode"] = .number(Double(min(100_000, max(0, item.episode))))
            value["artwork"] = .string("/art/\(item.id)?variant=episode")
            if item.hasBackdrop || item.seriesHasBackdrop { value["backdrop"] = .string("/backdrop/\(item.id)") }
            value["stream"] = .string("/media/\(item.id)")
        }
        set("container", item.container, max: 32)
        if item.size > 0 { value["size"] = .number(Double(min(item.size, 16_000_000_000_000))) }
        if !item.cast.isEmpty {
            value["cast"] = .array(item.cast.prefix(64).enumerated().map { index, person in
                .object(["name": .string(String(person.name.prefix(256))), "role": .string(String(person.role.prefix(512))),
                         "image": .string(person.image == nil ? "" : "/person/\(item.id)/\(index)")])
            })
        }
        return .object(value)
    }

    static func progress(_ state: DeviceUserState, mark: ProgressMark) -> JSONValue {
        .object(["seconds": .number(max(0, min(state.seconds, 315_360_000))), "watched": .bool(state.watched),
                 "session": .string(mark.session), "revision": .number(Double(mark.revision)), "dismissed": .bool(false)])
    }

    static func page(_ items: [DeviceItem], request: DeviceRequest, sorted: Bool, marks: [String: ProgressMark]) throws -> JSONValue {
        let offset = try request.number("offset", fallback: 0, range: 0...1_000_000)
        let limit = try request.number("limit", fallback: 60, range: 1...200)
        let view = request.query["view"] ?? "all"
        let sort = request.query["sort"] ?? "title"
        let query = request.query["q"] ?? ""
        guard LibraryView(rawValue: view) != nil, LibrarySort(rawValue: sort) != nil, query.utf8.count <= 512 else { throw DeviceFailure.invalid }
        var ordered = items
        if !sorted {
            switch sort {
            case "added": ordered.sort { ($0.added ?? .distantPast, $1.id) > ($1.added ?? .distantPast, $0.id) }
            case "year": ordered.sort { ($0.year, $1.id) > ($1.year, $0.id) }
            default: ordered.sort { titleOrder($0, $1) }
            }
        }
        var letters: [JSONValue] = []
        if sort == "title", query.isEmpty, view != "history", !sorted {
            letters = buckets(ordered)
        }
        var start = offset
        if let letter = request.query["letter"], !letter.isEmpty {
            guard let bucket = letters.first(where: { if case .object(let entry) = $0, entry["label"] == .string(letter) { true } else { false } }),
                  case .object(let entry) = bucket, case .number(let first)? = entry["offset"] else { throw DeviceFailure.invalid }
            if request.query["offset"] == nil { start = Int(first) }
        }
        let window = start < ordered.count ? Array(ordered[start..<min(ordered.count, start + limit)]) : []
        return .object(["items": .array(window.map { item($0, mark: marks[$0.id] ?? ProgressMark()) }), "total": .number(Double(ordered.count)),
                        "offset": .number(Double(offset)), "limit": .number(Double(limit)), "view": .string(view), "sort": .string(sort),
                        "query": .string(query), "letters": .array(letters)])
    }

    static func titleOrder(_ left: DeviceItem, _ right: DeviceItem) -> Bool {
        let leftLabel = label(left), rightLabel = label(right)
        if leftLabel != rightLabel { return leftLabel == "#" || rightLabel != "#" && leftLabel < rightLabel }
        let comparison = left.sortTitle.localizedStandardCompare(right.sortTitle)
        return comparison == .orderedAscending || comparison == .orderedSame && left.id < right.id
    }

    static func label(_ item: DeviceItem) -> String {
        guard let first = item.sortTitle.folding(options: [.diacriticInsensitive, .caseInsensitive], locale: nil).first, first.isLetter,
              first.unicodeScalars.allSatisfy({ CharacterSet.letters.contains($0) }) else { return "#" }
        return String(first).uppercased()
    }

    private static func buckets(_ items: [DeviceItem]) -> [JSONValue] {
        var result: [(label: String, offset: Int, count: Int)] = []
        for (offset, item) in items.enumerated() {
            let label = label(item)
            if result.last?.label == label { result[result.count - 1].count += 1 }
            else {
                guard !result.contains(where: { $0.label == label }) else { return [] }
                result.append((label, offset, 1))
            }
        }
        return result.map { .object(["label": .string($0.label), "offset": .number(Double($0.offset)), "count": .number(Double($0.count))]) }
    }
}
