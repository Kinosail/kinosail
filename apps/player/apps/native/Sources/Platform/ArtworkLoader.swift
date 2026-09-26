import Foundation
import ImageIO
import UniformTypeIdentifiers

actor ArtworkLoader {
    private struct Key: Hashable { let session: UUID; let path: String; let dimension: Int }
    private struct Cached { let image: CGImage; var used: Date; let saved: Date }
    private struct Loaded { let image: CGImage; let stale: Bool }
    private var cache: [Key: Cached] = [:]
    private var pending: [Key: Task<Loaded, Error>] = [:]
    private var refreshing: [Key: UUID] = [:]
    private var refreshTasks: [Key: Task<Void, Never>] = [:]
    private var bytes = 0
    private var generation = UUID()
    private var active = 0
    private var waiters: [(UUID, CheckedContinuation<Void, Error>)] = []

    func image(path: String, client: ServerClient, dimension: Int = 1600) async throws -> CGImage {
        try Task.checkCancellation()
        guard [400, 800, 1600, 4096].contains(dimension) else { throw ClientError.invalidInput("The artwork size is invalid.") }
        let url = try client.server.mediaURL(path)
        guard ["/art/", "/backdrop/", "/person/", "/media/"].contains(where: { url.path.hasPrefix($0) }) else {
            throw ClientError.invalidResponse
        }
        let key = Key(session: client.identity, path: url.absoluteString, dimension: dimension)
        if var found = cache[key] {
            found.used = Date(); cache[key] = found
            if Date().timeIntervalSince(found.saved) >= LocalMediaCache.artworkFreshLifetime {
                scheduleRefresh(key: key, url: url, client: client, dimension: dimension)
            }
            return found.image
        }
        let attempt = generation
        if let request = pending[key] {
            let loaded = try await request.value
            try Task.checkCancellation()
            guard generation == attempt else { throw CancellationError() }
            if loaded.stale { scheduleRefresh(key: key, url: url, client: client, dimension: dimension) }
            return loaded.image
        }
        guard pending.count < 64 else { throw ClientError.unavailable }
        let request = Task {
            defer { if generation == attempt { pending[key] = nil } }
            try Task.checkCancellation()
            let persist = URLComponents(url: url, resolvingAgainstBaseURL: false).map(ServerClient.cacheableMediaURL) == true
            let store = persist ? try await client.cacheStore() : nil
            let saved = await store?.read(url.absoluteString, kind: .artwork)
            if let saved, let image = try? Self.decodedThumbnail(saved.data, dimension: dimension) {
                try Task.checkCancellation()
                guard generation == attempt else { throw CancellationError() }
                remember(image, key: key, saved: saved.saved)
                return Loaded(image: image, stale: !saved.fresh)
            }
            if saved != nil { await store?.remove(url.absoluteString, kind: .artwork) }
            // Local hits must not queue behind slow network artwork requests.
            try await self.acquire()
            defer { self.release() }
            let (data, type) = try await client.resource(url.absoluteString, maximum: 32 * 1024 * 1024)
            guard type.hasPrefix("image/") else { throw ClientError.invalidResponse }
            let image = try Self.decodedThumbnail(data, dimension: dimension)
            try Task.checkCancellation()
            let savedAt = Date()
            guard generation == attempt else { throw CancellationError() }
            remember(image, key: key, saved: savedAt)
            if let store {
                await store.enqueueWrite(data, key: url.absoluteString, kind: .artwork)
            }
            return Loaded(image: image, stale: false)
        }
        pending[key] = request
        let loaded = try await request.value
        try Task.checkCancellation()
        guard generation == attempt else { throw CancellationError() }
        if loaded.stale { scheduleRefresh(key: key, url: url, client: client, dimension: dimension) }
        return loaded.image
    }

    static func decodedThumbnail(_ data: Data, dimension: Int) throws -> CGImage {
        guard [400, 800, 1600, 4096].contains(dimension), data.count <= 32 * 1024 * 1024 else { throw ClientError.invalidResponse }
        guard let source = CGImageSourceCreateWithData(data as CFData, [kCGImageSourceShouldCache: false] as CFDictionary),
              CGImageSourceGetCount(source) <= 256,
              let properties = CGImageSourceCopyPropertiesAtIndex(source, 0, nil) as? [CFString: Any],
              let width = properties[kCGImagePropertyPixelWidth] as? Int,
              let height = properties[kCGImagePropertyPixelHeight] as? Int,
              width > 0, height > 0, width <= 32_768, height <= 32_768, width * height <= 80_000_000,
              let image = CGImageSourceCreateThumbnailAtIndex(source, 0, [
                kCGImageSourceCreateThumbnailFromImageAlways: true,
                kCGImageSourceCreateThumbnailWithTransform: true,
                kCGImageSourceThumbnailMaxPixelSize: dimension,
                kCGImageSourceShouldCacheImmediately: true
              ] as CFDictionary) else { throw ClientError.invalidResponse }
        return image
    }

    // Reader resources need encoded bytes; native artwork keeps decoded pixels.
    static func thumbnail(_ data: Data, dimension: Int) throws -> Data {
        let image = try decodedThumbnail(data, dimension: dimension)
        let output = NSMutableData()
        let alpha = ![CGImageAlphaInfo.none, .noneSkipFirst, .noneSkipLast].contains(image.alphaInfo)
        guard let destination = CGImageDestinationCreateWithData(output, (alpha ? UTType.png.identifier : UTType.jpeg.identifier) as CFString, 1, nil) else {
            throw ClientError.invalidResponse
        }
        CGImageDestinationAddImage(destination, image, [kCGImageDestinationLossyCompressionQuality: 0.88] as CFDictionary)
        guard CGImageDestinationFinalize(destination), output.length <= 32 * 1024 * 1024 else { throw ClientError.invalidResponse }
        return output as Data
    }

    func clear() {
        generation = UUID()
        for task in pending.values { task.cancel() }
        for task in refreshTasks.values { task.cancel() }
        pending = [:]
        refreshing = [:]
        refreshTasks = [:]
        cache = [:]
        bytes = 0
    }

    private func scheduleRefresh(key: Key, url: URL, client: ServerClient, dimension: Int) {
        guard refreshing[key] == nil else { return }
        let attempt = generation
        refreshing[key] = attempt
        refreshTasks[key] = Task {
            await self.refresh(key: key, url: url, client: client, dimension: dimension, attempt: attempt)
        }
    }

    private func refresh(key: Key, url: URL, client: ServerClient, dimension: Int, attempt: UUID) async {
        defer {
            if refreshing[key] == attempt {
                refreshing[key] = nil
                refreshTasks[key] = nil
            }
        }
        guard generation == attempt else { return }
        do {
            try await acquire()
            defer { release() }
            try Task.checkCancellation()
            let (data, type) = try await client.resource(url.absoluteString, maximum: 32 * 1024 * 1024)
            guard type.hasPrefix("image/") else { throw ClientError.invalidResponse }
            let image = try Self.decodedThumbnail(data, dimension: dimension)
            try Task.checkCancellation()
            guard generation == attempt else { return }
            let savedAt = Date()
            let store = try await client.cacheStore()
            remember(image, key: key, saved: savedAt)
            if let store {
                await store.enqueueWrite(data, key: url.absoluteString, kind: .artwork)
            }
        } catch is CancellationError {} catch {}
    }

    private func remember(_ image: CGImage, key: Key, saved: Date) {
        // Account for decoded pixels, not the much smaller compressed file.
        let cost = image.bytesPerRow * image.height
        guard cost <= 32 * 1024 * 1024 else { return }
        if let previous = cache.removeValue(forKey: key) {
            bytes -= previous.image.bytesPerRow * previous.image.height
        }
        while bytes + cost > 32 * 1024 * 1024 || cache.count >= 96 {
            guard let oldest = cache.min(by: { $0.value.used < $1.value.used }) else { break }
            bytes -= oldest.value.image.bytesPerRow * oldest.value.image.height
            cache[oldest.key] = nil
        }
        cache[key] = Cached(image: image, used: Date(), saved: saved)
        bytes += cost
    }

    private func acquire() async throws {
        let id = UUID()
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
                if Task.isCancelled { continuation.resume(throwing: CancellationError()) }
                else if active < 4 { active += 1; continuation.resume() }
                else { waiters.append((id, continuation)) }
            }
        } onCancel: { Task { await self.cancel(id) } }
    }
    private func cancel(_ id: UUID) {
        if let index = waiters.firstIndex(where: { $0.0 == id }) { waiters.remove(at: index).1.resume(throwing: CancellationError()) }
    }
    private func release() {
        if waiters.isEmpty { active -= 1 }
        else { waiters.removeFirst().1.resume() }
    }
}
