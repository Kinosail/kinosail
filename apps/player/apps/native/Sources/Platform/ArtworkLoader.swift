import CoreGraphics
import Foundation

actor ArtworkLoader {
    private typealias Key = ArtworkMemoryIndex.Key
    private struct Cached { let image: CGImage; var used: Date; let saved: Date; var background: Bool }
    private struct Loaded { let image: CGImage; let stale: Bool }
    private struct Pending {
        let id: UUID
        let task: Task<Loaded, Error>
        var observers: Set<UUID>
    }
    private var cache: [Key: Cached] = [:]
    private var pending: [Key: Pending] = [:]
    private let downloads = ArtworkDownloads()
    private let warmup = ArtworkWarmup()
    private var refreshing: [Key: UUID] = [:]
    private var refreshTasks: [Key: Task<Void, Never>] = [:]
    private var bytes = 0
    private var generation = UUID()
    private var active = 0
    private var activeBackground = 0
    private var foreground: [String: Int] = [:]
    private var visible: [Key: Int] = [:]
    private var waiters: [(id: UUID, key: String, background: Bool, continuation: CheckedContinuation<Bool, Error>)] = []
    nonisolated let recent = ArtworkMemoryIndex()
    private let memoryBudget: Int
    private let entryLimit: Int

    init(memoryBudget: Int = ArtworkLoader.imageLimit, entryLimit: Int = 96) {
        self.memoryBudget = memoryBudget
        self.entryLimit = entryLimit
    }

    nonisolated func cachedImage(path: String, client: ServerClient, dimension: Int) -> CGImage? {
        recent.image(for: Key(session: client.identity, path: path, dimension: dimension))
    }

    func prefetch(paths: [String], client: ServerClient, dimension: Int = 800) async throws {
        try await warmup.prefetch(paths: paths, client: client, dimension: dimension, loader: self)
    }
    func stopPrefetch() async { await warmup.clear() }

    func image(path: String, client: ServerClient, dimension: Int = 1600, background: Bool = false) async throws -> CGImage {
        try Task.checkCancellation()
        guard [400, 500, 800, 1600, 4096].contains(dimension) else { throw ClientError.invalidInput("The artwork size is invalid.") }
        let url = try client.server.mediaURL(path)
        guard ["/art/", "/episode-art/", "/backdrop/", "/person/", "/media/"].contains(where: { url.path.hasPrefix($0) }) else {
            throw ClientError.invalidResponse
        }
        let key = Key(session: client.identity, path: url.absoluteString, dimension: dimension)
        recent.alias(Key(session: client.identity, path: path, dimension: dimension), to: key)
        if var found = cache[key] {
            if !background { found.background = false }
            found.used = Date(); cache[key] = found
            if Date().timeIntervalSince(found.saved) >= LocalMediaCache.artworkFreshLifetime {
                scheduleRefresh(key: key, url: url, client: client, dimension: dimension)
            }
            return found.image
        }
        let networkKey = "\(client.identity):\(url.absoluteString)"
        if !background {
            visible[key, default: 0] += 1
            foreground[networkKey, default: 0] += 1
            for index in waiters.indices where waiters[index].key == networkKey { waiters[index].background = false }
            drain()
        }
        defer {
            if !background {
                visible[key, default: 1] -= 1
                if visible[key] == 0 { visible[key] = nil }
                foreground[networkKey, default: 1] -= 1
                if foreground[networkKey] == 0 { foreground[networkKey] = nil }
            }
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
                let storageKey = Self.artworkStorageKey(url: url, dimension: dimension)
                let saved = await store?.read(storageKey, kind: .artwork)
                if let saved {
                    do {
                        let image = try await Self.decode(saved.data, dimension: dimension)
                        try Task.checkCancellation()
                        guard generation == attempt else { throw CancellationError() }
                        remember(image, key: key, saved: saved.saved, background: background && visible[key] == nil)
                        return Loaded(image: image, stale: !saved.fresh)
                    } catch is CancellationError { throw CancellationError() }
                    catch { await store?.remove(storageKey, kind: .artwork) }
                }
                let (data, type): (Data, String)
                do { (data, type) = try await download(url: url, client: client, background: background) }
                catch {
                    if let image = try await Self.legacyBackdropImage(after: error, url: url, dimension: dimension, store: store) {
                        return Loaded(image: image, stale: false)
                    }
                    throw error
                }
                guard type.hasPrefix("image/") else { throw ClientError.invalidResponse }
                let image = try await Self.decode(data, dimension: dimension)
                try Task.checkCancellation()
                let savedAt = Date()
                guard generation == attempt else { throw CancellationError() }
                remember(image, key: key, saved: savedAt, background: background && visible[key] == nil)
                if let store {
                    await store.enqueueWrite(data, key: storageKey, kind: .artwork)
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

    func clear() async {
        generation = UUID()
        for request in pending.values { request.task.cancel() }
        for task in refreshTasks.values { task.cancel() }
        pending = [:]
        refreshing = [:]
        refreshTasks = [:]
        cache = [:]
        bytes = 0
        recent.clear()
        await warmup.clear()
        await downloads.clear()
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
            try Task.checkCancellation()
            let (data, type) = try await download(url: url, client: client, background: true)
            guard type.hasPrefix("image/") else { throw ClientError.invalidResponse }
            let image = try await Self.decode(data, dimension: dimension)
            try Task.checkCancellation()
            guard generation == attempt else { return }
            let savedAt = Date()
            let persist = URLComponents(url: url, resolvingAgainstBaseURL: false).map(ServerClient.cacheableMediaURL) == true
            let store = persist ? try await client.cacheStore() : nil
            let storageKey = Self.artworkStorageKey(url: url, dimension: dimension)
            remember(image, key: key, saved: savedAt, background: true)
            if let store {
                await store.enqueueWrite(data, key: storageKey, kind: .artwork)
            }
        } catch is CancellationError {} catch {}
    }

    private func remember(_ image: CGImage, key: Key, saved: Date, background: Bool = false) {
        let cost = image.bytesPerRow * image.height
        guard cost <= Self.imageLimit else { forget(key); return }
        let retainedBackground = background && cache[key]?.background != false
        if background {
            let protected = cache.lazy.filter { $0.key != key && !$0.value.background }
            guard protected.count < entryLimit,
                  protected.reduce(cost, { $0 + $1.value.image.bytesPerRow * $1.value.image.height }) <= memoryBudget else { forget(key); return }
        }
        forget(key)
        while bytes + cost > memoryBudget || cache.count >= entryLimit {
            let oldest = cache.lazy.filter { $0.value.background }.min(by: { $0.value.used < $1.value.used })
                ?? (background ? nil : cache.min(by: { $0.value.used < $1.value.used }))
            guard let oldest else { return }
            forget(oldest.key)
        }
        cache[key] = Cached(image: image, used: Date(), saved: saved, background: retainedBackground)
        recent.set(image, for: key)
        bytes += cost
    }

    private func forget(_ key: Key) {
        if let previous = cache.removeValue(forKey: key) { bytes -= previous.image.bytesPerRow * previous.image.height }
        recent.remove(key)
    }

    private func download(url: URL, client: ServerClient, background: Bool) async throws -> (Data, String) {
        try await downloads.resource(path: url.absoluteString, clientID: client.identity) {
            try await self.fetch(url: url, client: client, background: background)
        }
    }

    private func fetch(url: URL, client: ServerClient, background: Bool) async throws -> (Data, String) {
        let admittedBackground = try await acquire(key: "\(client.identity):\(url.absoluteString)", background: background)
        defer { release(background: admittedBackground) }
        try Task.checkCancellation()
        return try await client.resource(url.absoluteString, maximum: 32 * 1024 * 1024)
    }

    private func acquire(key: String, background: Bool) async throws -> Bool {
        let id = UUID()
        return try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Bool, Error>) in
                if Task.isCancelled { continuation.resume(throwing: CancellationError()) }
                else { waiters.append((id, key, background && foreground[key] == nil, continuation)); drain() }
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
        drain()
    }
    private func drain() {
        while active < 4 {
            guard let next = waiters.firstIndex(where: { !$0.background }) ??
                    (activeBackground < 2 ? waiters.firstIndex(where: { $0.background }) : nil) else { return }
            let waiter = waiters.remove(at: next)
            active += 1
            if waiter.background { activeBackground += 1 }
            waiter.continuation.resume(returning: waiter.background)
        }
    }
}
