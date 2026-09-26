import CoreGraphics
import Foundation
import ImageIO
import Testing
import UniformTypeIdentifiers
@testable import KinosailPlayer

struct EpisodeStillArtworkTests {
    @Test func loadsAndCachesAnEpisodeStill() async throws {
        let context = try #require(CGContext(data: nil, width: 2, height: 2, bitsPerComponent: 8, bytesPerRow: 0,
                                             space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.setFillColor(CGColor(red: 0.2, green: 0.8, blue: 0.4, alpha: 1))
        context.fill(CGRect(x: 0, y: 0, width: 2, height: 2))
        let image = try #require(context.makeImage())
        let data = NSMutableData()
        let destination = try #require(CGImageDestinationCreateWithData(data, UTType.png.identifier as CFString, 1, nil))
        CGImageDestinationAddImage(destination, image, nil)
        #expect(CGImageDestinationFinalize(destination))

        let fixture = try HTTPFixture(data: data as Data, headers: ["Content-Type": "image/png"])
        defer { fixture.remove() }
        let loader = ArtworkLoader()
        let first = try await loader.image(path: "/episode-art/first", client: fixture.client, dimension: 400)
        let cached = try await loader.image(path: "/episode-art/first", client: fixture.client, dimension: 400)
        #expect(first === cached)
        #expect(fixture.requests.count == 1)
        #expect(fixture.requests.first?.url?.path == "/episode-art/first")
    }
}
