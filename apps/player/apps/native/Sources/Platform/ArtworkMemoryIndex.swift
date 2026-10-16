import CoreGraphics
import Foundation
import Synchronization

final class ArtworkMemoryIndex: Sendable {
    struct Key: Hashable, Sendable {
        let session: UUID
        let path: String
        let dimension: Int
    }
    private struct State {
        var images: [Key: CGImage] = [:]
        var aliases: [Key: Key] = [:]
    }
    private let state = Mutex(State())

    func image(for key: Key) -> CGImage? {
        state.withLock { state in
            state.images[key] ?? state.aliases[key].flatMap { state.images[$0] }
        }
    }

    func alias(_ alias: Key, to key: Key) {
        guard alias != key else { return }
        state.withLock { state in
            if state.aliases.count >= 2_048 { state.aliases.removeAll(keepingCapacity: true) }
            state.aliases[alias] = key
        }
    }

    func set(_ image: CGImage, for key: Key) {
        state.withLock { $0.images[key] = image }
    }

    func remove(_ key: Key) {
        state.withLock { $0.images[key] = nil }
    }

    func clear() {
        state.withLock { state in
            state.images.removeAll()
            state.aliases.removeAll()
        }
    }
}
