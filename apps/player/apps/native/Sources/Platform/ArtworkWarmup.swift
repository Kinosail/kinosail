import Foundation

/// A bounded queue keeps look-ahead work behind visible artwork and out of catalog requests.
actor ArtworkWarmup {
    private struct Request {
        let path: String
        let client: ServerClient
        let dimension: Int
    }
    private var queue: [Request] = []
    private var task: Task<Void, Never>?
    private var generation = UUID()

    func prefetch(paths: [String], client: ServerClient, dimension: Int, loader: ArtworkLoader) throws {
        try Task.checkCancellation()
        guard paths.count <= 24, [400, 500, 800, 1600].contains(dimension) else { throw ClientError.invalidResponse }
        // Validate the entire batch before admitting any background work.
        let urls = try paths.map { path in
            guard !path.isEmpty, path.utf8.count <= 16_384,
                  path.rangeOfCharacter(from: .controlCharacters) == nil else { throw ClientError.invalidResponse }
            let url = try client.server.mediaURL(path)
            guard ["/art/", "/episode-art/", "/backdrop/", "/person/"].contains(where: { url.path.hasPrefix($0) }),
                  let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
                  ServerClient.cacheableMediaURL(components) else { throw ClientError.invalidResponse }
            return url.absoluteString
        }
        var seen: Set<String> = []
        for path in urls where seen.insert(path).inserted {
            guard queue.count < 64 else { break }
            if !queue.contains(where: { $0.client.identity == client.identity && $0.path == path && $0.dimension == dimension }) {
                queue.append(Request(path: path, client: client, dimension: dimension))
            }
        }
        guard task == nil, !queue.isEmpty else { return }
        let attempt = generation
        task = Task(priority: .utility) {
            defer { if generation == attempt { task = nil } }
            while generation == attempt && !Task.isCancelled && !queue.isEmpty {
                let next = queue.removeFirst()
                _ = try? await loader.image(path: next.path, client: next.client, dimension: next.dimension, background: true)
            }
        }
    }

    func clear() {
        generation = UUID()
        task?.cancel()
        task = nil
        queue = []
    }
}
