import Foundation

struct DeviceLibraryWatched {
    let catalog: DeviceCatalog

    func item(_ id: String, request: DeviceRequest) async throws -> DeviceResponse {
        let watched = try DeviceLibraryRouter.body(request) { try $0.object(allowing: ["watched"]).flag("watched") }
        guard let item = try await catalog.lookup(id), item.kind != .series else { throw DeviceFailure.notFound }
        try await mark([item], watched: watched)
        return .json(200, .object(["watched": .bool(watched), "items": .number(1)]))
    }

    func show(_ id: String, request: DeviceRequest) async throws -> DeviceResponse {
        let (watched, season) = try DeviceLibraryRouter.body(request) { raw in
            let value = try raw.object(allowing: ["watched", "season"])
            let season = value["season"] == nil ? nil : Int(try value.number("season", max: 9999, integer: true))
            return (try value.flag("watched"), season)
        }
        let (_, episodes) = try await catalog.show(id)
        let selected = season.map { number in episodes.filter { $0.season == number } } ?? episodes
        guard !selected.isEmpty else { throw DeviceFailure.notFound }
        try await mark(selected, watched: watched)
        return .json(200, .object(["watched": .bool(watched), "items": .number(Double(selected.count))]))
    }

    private func mark(_ items: [DeviceItem], watched: Bool) async throws {
        for item in items {
            for version in item.versions.isEmpty ? [item.ref] : item.versions {
                try await catalog.client(for: version).setPosition(version.item, seconds: 0, watched: watched)
            }
            await catalog.updateState(item.id) { $0.seconds = 0; $0.watched = watched }
        }
    }
}
