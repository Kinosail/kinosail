import Foundation

extension ServerClient {
    func actor(name: String, policy: CatalogPolicy = .reload) async throws -> ActorDetail {
        let name = try Input.actorName(name)
        return try await catalog("/api/v1/actor?name=\(Input.segment(name))", policy: policy) { [self] raw in
            try ActorDetail(raw, name: name, server: server)
        }
    }
}
