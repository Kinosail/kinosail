#if os(iOS)
import Foundation

actor ReaderResourceLoader {
    private var active = 0
    private var usedBytes = 0
    private var reservedBytes = 0
    private var requests = 0
    private var waiters: [(UUID, CheckedContinuation<Void, Error>)] = []

    func load(url: URL, itemID: String, client: ServerClient) async throws -> (Data, String) {
        let remote = try ReaderResourcePolicy.remote(url.absoluteString, itemID: itemID, server: client.server)
        guard requests < 512 else { throw ClientError.invalidInput("This chapter contains too many files to open. Try another chapter.") }
        requests += 1
        try await acquire()
        defer { release() }
        try Task.checkCancellation()
        let maximum = min(32 * 1024 * 1024, 64 * 1024 * 1024 - usedBytes - reservedBytes)
        guard maximum > 0 else { throw ClientError.invalidInput("This chapter is too large to open safely.") }
        reservedBytes += maximum
        var received = false
        do {
            let (data, mime) = try await client.resource(remote.absoluteString, maximum: maximum)
            reservedBytes -= maximum; usedBytes += data.count; received = true
            let allowed = ["text/html", "application/xhtml+xml", "text/css", "text/plain", "application/pdf", "application/font-woff", "application/vnd.ms-opentype", "application/octet-stream", "image/svg+xml"]
            guard allowed.contains(mime) || mime.hasPrefix("image/") || mime.hasPrefix("font/") else { throw ClientError.invalidResponse }
            if remote.path.hasSuffix("/file"), mime != "application/pdf" { throw ClientError.invalidResponse }
            if mime.hasPrefix("image/"), mime != "image/svg+xml" {
                let image = try ArtworkLoader.thumbnail(data, dimension: 4096)
                return (image, image.first == 0x89 ? "image/png" : "image/jpeg")
            }
            return (data, mime)
        } catch {
            // Failed transfers consume their reservation too, bounding repeated
            // broken subresource requests without guessing partial byte counts.
            if !received { reservedBytes -= maximum; usedBytes += maximum }
            throw error
        }
    }

    private func acquire() async throws {
        try Task.checkCancellation()
        let id = UUID()
        try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
                if Task.isCancelled { continuation.resume(throwing: CancellationError()) }
                else if active < 2 { active += 1; continuation.resume() }
                else if waiters.count < 128 { waiters.append((id, continuation)) }
                else { continuation.resume(throwing: ClientError.invalidInput("This chapter needs to load too many files at once. Try another chapter.")) }
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
#endif
