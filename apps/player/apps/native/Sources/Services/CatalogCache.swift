import Foundation

enum CatalogPolicy: Sendable { case cached, automatic, reload }
enum CatalogCacheMiss: Error { case missing }
struct CatalogRequest: Sendable {
    let id: UUID
    let task: Task<any Sendable, Error>
}

extension ServerClient {
    /// Domain validation runs on both remote and persisted data. This seam is
    /// deliberately limited to catalog reads, never playback or authentication.
    func catalog<Value: Sendable>(_ path: String, policy: CatalogPolicy,
                                 decode: @escaping (JSONValue) throws -> Value) async throws -> Value {
        try Task.checkCancellation()
        let url = try authorizedRequest(path).url!
        guard let encodedPath = URLComponents(url: url, resolvingAgainstBaseURL: false)?.percentEncodedPath else {
            throw ClientError.invalidInput("The catalog request is invalid.")
        }
        let parts = encodedPath.split(separator: "/")
        guard parts.count >= 3, parts[0] == "api", parts[1] == "v1",
              parts.count == 3 && ["library", "actor", "collections", "albums"].contains(parts[2]) ||
              parts.count == 4 && ["items", "shows", "collections", "albums"].contains(parts[2]),
              ["library", "actor"].contains(parts[2]) || url.query == nil else { throw ClientError.invalidInput("The catalog request is invalid.") }
        let generation = catalogGeneration
        let store = try cacheStore()
        let cacheRevision = await store?.revision
        let pagesRevision = await store?.pagesRevision
        let offset = URLComponents(url: url, resolvingAgainstBaseURL: false)?.queryItems?.first { $0.name == "offset" }?.value
        let entry = await store?.read(path, kind: .catalog)
        let previous = entry.flatMap { try? StrictJSON.decode($0.data) }
        if policy != .reload, let entry, let previous {
            if Self.cacheableCatalog(previous), let value = try? decode(previous) {
                try Task.checkCancellation()
                guard catalogGeneration == generation else { throw CancellationError() }
                if policy == .cached || entry.fresh { return value }
            } else { await store?.remove(path, kind: .catalog) }
        }
        try Task.checkCancellation()
        guard catalogGeneration == generation else { throw CancellationError() }
        if policy == .cached { throw CatalogCacheMiss.missing }
        let request: CatalogRequest
        if let pending = catalogRequests[path] { request = pending }
        else {
            guard catalogRequests.count < 32 else { throw ClientError.unavailable }
            let id = UUID()
            // The producer owns persistence. Leaving a screen must not throw away
            // a successful fetch shared with another screen or a future visit.
            request = CatalogRequest(id: id, task: Task {
                defer { if catalogRequests[path]?.id == id { catalogRequests[path] = nil } }
                do {
                    let raw = try await self.request(path).body
                    try Task.checkCancellation()
                    guard catalogGeneration == generation else { throw CancellationError() }
                    let value = try decode(raw)
                    if let previous, previous != raw, parts[2] == "library", offset == "0" {
                        await store?.invalidatePages()
                    }
                    try Task.checkCancellation()
                    guard catalogGeneration == generation else { throw CancellationError() }
                    if Self.cacheableCatalog(raw), let data = try? JSONEncoder().encode(raw) {
                        if let store {
                            await store.enqueueWrite(data, key: path, kind: .catalog, revision: cacheRevision,
                                                     pagesRevision: offset != nil && offset != "0" ? pagesRevision : nil)
                        }
                    } else { await store?.remove(path, kind: .catalog) }
                    return value
                } catch {
                    if error as? ClientError == .http(401) || error as? ClientError == .http(403) { await discardMediaCache() }
                    else if error as? ClientError == .http(404) { await store?.remove(path, kind: .catalog) }
                    throw error
                }
            })
            catalogRequests[path] = request
        }
        let result = try await request.task.value
        try Task.checkCancellation()
        guard catalogGeneration == generation else { throw CancellationError() }
        guard let value = result as? Value else { throw ClientError.invalidResponse }
        return value
    }

    func invalidateCatalog() async {
        catalogGeneration = UUID()
        for request in catalogRequests.values { request.task.cancel() }
        catalogRequests = [:]
        if let store = try? cacheStore() { await store.invalidateCatalog() }
    }

    private static func cacheableCatalog(_ value: JSONValue) -> Bool {
        switch value {
        case .object(let fields):
            for (name, value) in fields {
                let key = name.lowercased()
                if key.contains("token") || key.contains("secret") || ["authorization", "password", "cookie"].contains(key) { return false }
                if ["stream", "download", "artwork", "poster", "backdrop", "play", "url", "href", "image"].contains(key),
                   case .string(let path) = value, !path.isEmpty {
                    guard path.hasPrefix("/"), !path.hasPrefix("//"), let url = URLComponents(string: path),
                          url.scheme == nil, url.host == nil, Self.cacheableMediaURL(url), url.fragment == nil else { return false }
                }
                if !cacheableCatalog(value) { return false }
            }
            return true
        case .array(let values): return values.allSatisfy(cacheableCatalog)
        default: return true
        }
    }

    static func cacheableMediaURL(_ url: URLComponents) -> Bool {
        url.query == nil || url.path.hasPrefix("/art/") && url.percentEncodedQuery == "variant=episode" ||
            url.path.hasPrefix("/person/") && url.percentEncodedQuery == "scope=show"
    }
}

extension ClientError {
    var discardsCachedContent: Bool { self == .http(401) || self == .http(403) || self == .http(404) }
}
