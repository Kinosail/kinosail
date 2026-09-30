import Foundation
import ImageIO
import Testing
@testable import KinosailPlayer

struct ArtworkSharingTests {
    @Test func sharesOneDownloadAcrossConcurrentArtworkSizes() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.hold = true }
        let loader = ArtworkLoader()
        let small = Task { try await loader.image(path: "/art/movie", client: fixture.client, dimension: 800) }
        let large = Task { try await loader.image(path: "/art/movie", client: fixture.client, dimension: 1600) }
        for _ in 0..<200 where fixture.requests.isEmpty { try await Task.sleep(for: .milliseconds(5)) }
        try await Task.sleep(for: .milliseconds(100))
        #expect(fixture.requests.count == 1)
        await loader.clear()
        await #expect(throws: CancellationError.self) { try await small.value }
        await #expect(throws: CancellationError.self) { try await large.value }
        await fixture.client.close()
    }

    @Test func cancellingOneSizePreservesTheOtherAndLastCancellationFreesCapacity() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let data = try artworkImageData(width: 800, height: 400)
        ControlledArtworkProtocol.entries.withLock { $0[fixture.host] = .init(data: data) }
        defer { ControlledArtworkProtocol.entries.withLock { $0[fixture.host] = nil } }
        let client = try ServerClient(server: await fixture.client.server, protocolClasses: [ControlledArtworkProtocol.self])
        let loader = ArtworkLoader()
        let small = Task { try await loader.image(path: "/art/movie", client: client, dimension: 400) }
        let large = Task { try await loader.image(path: "/art/movie", client: client, dimension: 800) }
        for _ in 0..<200 where ControlledArtworkProtocol.active(fixture.host).isEmpty { try await Task.sleep(for: .milliseconds(5)) }
        try await Task.sleep(for: .milliseconds(50))
        small.cancel()
        try await Task.sleep(for: .milliseconds(50))
        let remaining = ControlledArtworkProtocol.active(fixture.host)
        #expect(remaining.count == 1)
        remaining.forEach { $0.finish() }
        await #expect(throws: CancellationError.self) { try await small.value }
        #expect(try await large.value.width == 800)
        #expect(ControlledArtworkProtocol.requests(fixture.host) == 1)

        let stalled = (0..<4).map { index in Task { try await loader.image(path: "/art/stalled\(index)", client: client) } }
        for _ in 0..<200 where ControlledArtworkProtocol.active(fixture.host).count < 4 { try await Task.sleep(for: .milliseconds(5)) }
        stalled.forEach { $0.cancel() }
        let next = Task { try await loader.image(path: "/art/next", client: client) }
        for _ in 0..<200 where !ControlledArtworkProtocol.active(fixture.host).contains(where: { $0.request.url?.path == "/art/next" }) {
            try await Task.sleep(for: .milliseconds(5))
        }
        let admitted = ControlledArtworkProtocol.active(fixture.host).filter { $0.request.url?.path == "/art/next" }
        #expect(admitted.count == 1)
        admitted.forEach { $0.finish() }
        if !admitted.isEmpty { #expect(try await next.value.width == 800) }
        else { next.cancel() }
        await loader.clear()
        for task in stalled { await #expect(throws: CancellationError.self) { try await task.value } }
        await client.close()
    }
}
