import Foundation
import Observation

@MainActor @Observable
final class MediaQueue {
    enum RepeatMode: String, CaseIterable, Sendable { case off, all, one }
    private(set) var items: [MediaItem] = []
    private(set) var currentIndex: Int?
    private(set) var shuffled = false
    private(set) var repeatMode = RepeatMode.off
    private var original: [MediaItem] = []

    func replace(_ items: [MediaItem], startingAt index: Int) throws {
        guard !items.isEmpty, items.count <= 2000, items.indices.contains(index), items.allSatisfy({ $0.kind == .music }), Set(items.map(\.id)).count == items.count else {
            throw ClientError.invalidInput("The music queue is invalid.")
        }
        self.items = items
        original = items
        currentIndex = index
        shuffled = false
    }

    func next(automatic: Bool = false) -> MediaItem? {
        guard let index = currentIndex else { return nil }
        if automatic, repeatMode == .one { return items[index] }
        let next = index + 1
        if items.indices.contains(next) { currentIndex = next; return items[next] }
        if repeatMode == .all { currentIndex = 0; return items.first }
        return nil
    }

    func previous() -> MediaItem? {
        guard let index = currentIndex else { return nil }
        let previous = max(0, index - 1)
        currentIndex = previous
        return items[previous]
    }

    func setShuffle(_ enabled: Bool) {
        guard enabled != shuffled, let index = currentIndex else { return }
        let current = items[index]
        items = enabled ? [current] + original.filter { $0.id != current.id }.shuffled() : original
        currentIndex = items.firstIndex { $0.id == current.id }
        shuffled = enabled
    }

    func setRepeat(_ mode: RepeatMode) { repeatMode = mode }
    func clear() { items = []; original = []; currentIndex = nil; shuffled = false; repeatMode = .off }
}
