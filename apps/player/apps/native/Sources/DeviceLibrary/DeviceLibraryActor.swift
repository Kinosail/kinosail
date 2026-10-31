import Foundation

struct DeviceLibraryActor {
    let catalog: DeviceCatalog

    func page(_ request: DeviceRequest) async throws -> DeviceResponse {
        guard let raw = request.query["name"], let name = try? Input.actorName(raw) else { throw DeviceFailure.notFound }
        let (items, image) = try await catalog.credits(name)
        func credits(_ kind: DeviceItem.Kind, route: String) -> JSONValue {
            .array(items.filter { $0.kind == kind }.prefix(500).map { item in
                .object(["id": .string(item.id), "title": .string(String(item.title.prefix(512))), "year": .string(String(item.year.prefix(16))),
                         "role": .string(""), "artwork": .string(item.hasPrimary ? "/art/\(item.id)" : ""), "url": .string(route + item.id)])
            })
        }
        return .json(200, .object(["name": .string(name), "image": .string(image),
                                   "movies": credits(.movie, route: "/watch/"), "shows": credits(.series, route: "/show/")]))
    }
}
