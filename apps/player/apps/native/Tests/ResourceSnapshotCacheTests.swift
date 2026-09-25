import Foundation
import Testing
@testable import KinosailPlayer

@MainActor
struct ResourceSnapshotCacheTests {
    @Test func reusesOnlyTheMatchingClientIdentityAndType() {
        let cache = ResourceSnapshotCache()
        let first = UUID(), second = UUID()
        cache.store("Watch", for: "home", clientID: first)
        cache.store(3, for: "home", clientID: first)
        #expect(cache.value(for: "home", clientID: first, as: String.self) == "Watch")
        #expect(cache.value(for: "home", clientID: first, as: Int.self) == 3)
        #expect(cache.value(for: "home", clientID: second, as: String.self) == nil)
        cache.clear()
        #expect(cache.value(for: "home", clientID: first, as: String.self) == nil)
    }

    @Test func boundsMemoryAndRejectsOversizedIdentityWithoutEviction() {
        let cache = ResourceSnapshotCache()
        let client = UUID()
        for index in 0..<64 { cache.store(index, for: "page:\(index)", clientID: client) }
        _ = cache.value(for: "page:0", clientID: client, as: Int.self)
        cache.store(64, for: "page:64", clientID: client)
        #expect(cache.value(for: "page:0", clientID: client, as: Int.self) == 0)
        #expect(cache.value(for: "page:1", clientID: client, as: Int.self) == nil)
        cache.store(65, for: String(repeating: "x", count: 1025), clientID: client)
        #expect(cache.value(for: "page:64", clientID: client, as: Int.self) == 64)
        cache.remove(for: "page:64", clientID: client, as: Int.self)
        #expect(cache.value(for: "page:64", clientID: client, as: Int.self) == nil)
    }

    @Test func refreshesInTheBackgroundOnlyAfterTheWindowOrRevisionChanges() {
        let cache = ResourceSnapshotCache()
        let client = UUID()
        cache.store("Listen", for: "home", clientID: client, refreshID: "revision-1")
        #expect(cache.isFresh(for: "home", clientID: client, as: String.self, refreshID: "revision-1"))
        #expect(!cache.isFresh(for: "home", clientID: client, as: String.self, refreshID: "revision-2"))
        #expect(!cache.isFresh(for: "home", clientID: client, as: String.self,
                               refreshID: "revision-1", now: Date().addingTimeInterval(61)))
        cache.store("Listen", for: "home", clientID: client)
        #expect(!cache.isFresh(for: "home", clientID: client, as: String.self, refreshID: "revision-1"))
        #expect(cache.value(for: "home", clientID: client, as: String.self) == "Listen")
    }
}
