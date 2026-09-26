import Foundation
import ImageIO
import UniformTypeIdentifiers

actor ArtworkLoader {
    private struct Key: Hashable, Sendable { let session: UUID; let path: String; let dimension: Int }
    private struct Cached { let image: CGImage; var used: Date; let saved: Date }
    private struct Loaded { let image: CGImage; let stale: Bool }
    private struct Pending {
        let id: UUID
        let task: Task<Loaded, Error>
        var observers: Set<UUID>
    }
    private var cache: [Key: Cached] = [:]
    private var pending: [Key: Pending] = [:]
    private var refreshing: [Key: UUID] = [:]
    private var refreshTasks: [Key: Task<Void, Never>] = [:]
    private var bytes = 0
    private var generation = UUID()
    private var active = 0
    private var activeBackground = 0
    private var waiters: [(id: UUID, background: Bool, continuation: CheckedContinuation<Void, Error>)] = []

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
        let observer = UUID()
        let request: Pending
        if var existing = pending[key] {
            existing.observers.insert(observer)
            pending[key] = existing
            request = existing
        } else {
            guard pending.count < 64 else { throw ClientError.unavailable }
            let id = UUID()
            let task = Task {
                defer { if pending[key]?.id == id { pending[key] = nil } }
                try Task.checkCancellation()
                let persist = URLComponents(url: url, resolvingAgainstBaseURL: false).map(ServerClient.cacheableMediaURL) == true
                let store = persist ? try await client.cacheStore() : nil
                let saved = await store?.read(url.absoluteString, kind: .artwork)
                if let saved {
                    do {
                        let image = try await Self.decode(saved.data, dimension: dimension)
                        try Task.checkCancellation()
                        guard generation == attempt else { throw CancellationError() }
                        remember(image, key: key, saved: saved.saved)
                        return Loaded(image: image, stale: !saved.fresh)
                    } catch is CancellationError { throw CancellationError() }
                    catch { await store?.remove(url.absoluteString, kind: .artwork) }
                }
                // Local hits must not queue behind slow network artwork requests.
                try await self.acquire()
                defer { self.release() }
                let (data, type) = try await client.resource(url.absoluteString, maximum: 32 * 1024 * 1024)
                guard type.hasPrefix("image/") else { throw ClientError.invalidResponse }
                let image = try await Self.decode(data, dimension: dimension)
                try Task.checkCancellation()
                let savedAt = Date()
                guard generation == attempt else { throw CancellationError() }
                remember(image, key: key, saved: savedAt)
                if let store {
                    await store.enqueueWrite(data, key: url.absoluteString, kind: .artwork)
                }
                return Loaded(image: image, stale: false)
            }
            request = Pending(id: id, task: task, observers: [observer])
            pending[key] = request
        }
        let requestID = request.id
        let loaded = try await withTaskCancellationHandler {
            try await request.task.value
        } onCancel: {
            Task { await self.stopWaiting(key: key, requestID: requestID, observerID: observer) }
        }
        try Task.checkCancellation()
        guard generation == attempt else { throw CancellationError() }
        if loaded.stale { scheduleRefresh(key: key, url: url, client: client, dimension: dimension) }
        return loaded.image
    }

    private func stopWaiting(key: Key, requestID: UUID, observerID: UUID) {
        guard var request = pending[key], request.id == requestID else { return }
        request.observers.remove(observerID)
        if request.observers.isEmpty {
            pending[key] = nil
            request.task.cancel()
        } else { pending[key] = request }
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

    private static func decode(_ data: Data, dimension: Int) async throws -> CGImage {
        let work = Task.detached(priority: Task.currentPriority) {
            try Task.checkCancellation()
            return try decodedThumbnail(data, dimension: dimension)
        }
        return try await withTaskCancellationHandler {
            try await work.value
        } onCancel: {
            work.cancel()
        }
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
        for request in pending.values { request.task.cancel() }
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
            try await acquire(background: true)
            defer { release(background: true) }
            try Task.checkCancellation()
            let (data, type) = try await client.resource(url.absoluteString, maximum: 32 * 1024 * 1024)
            guard type.hasPrefix("image/") else { throw ClientError.invalidResponse }
            let image = try await Self.decode(data, dimension: dimension)
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

    private func acquire(background: Bool = false) async throws {
        let id = UUID()
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
                if Task.isCancelled { continuation.resume(throwing: CancellationError()) }
                else if active < 4 && (!background || activeBackground < 2) {
                    active += 1
                    if background { activeBackground += 1 }
                    continuation.resume()
                } else { waiters.append((id, background, continuation)) }
            }
        } onCancel: { Task { await self.cancel(id) } }
    }
    private func cancel(_ id: UUID) {
        if let index = waiters.firstIndex(where: { $0.id == id }) {
            waiters.remove(at: index).continuation.resume(throwing: CancellationError())
        }
    }
    private func release(background: Bool = false) {
        active -= 1
        if background { activeBackground -= 1 }
        let next = waiters.firstIndex(where: { !$0.background }) ??
            (activeBackground < 2 ? waiters.firstIndex(where: { $0.background }) : nil)
        if let next {
            let waiter = waiters.remove(at: next)
            active += 1
            if waiter.background { activeBackground += 1 }
            waiter.continuation.resume()
        }
    }
}
