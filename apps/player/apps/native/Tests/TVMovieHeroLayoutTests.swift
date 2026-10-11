#if os(tvOS)
import SwiftUI
import Testing
@testable import KinosailPlayer

// An isolated painted-layout check catches inset/cropped artwork and clipped
// large content. Remote E2E tests still own real focus and navigation proof.
struct TVMovieHeroLayoutTests {
    private let red = Color(.sRGB, red: 1, green: 0, blue: 0, opacity: 1)
    private let white = Color(.sRGB, red: 1, green: 1, blue: 1, opacity: 1)
    private let black = Color(.sRGB, red: 0, green: 0, blue: 0, opacity: 1)
    @Test @MainActor func artworkReachesScreenEdgesWhileContentStaysInset() throws {
        let renderer = ImageRenderer(content: VStack(spacing: 0) {
            TVMovieHeroLayout { white.frame(height: 300) }
                .background(red).tvMovieHeroBleed()
        }
        .padding(KinoTheme.contentPadding).frame(width: 1920).background(black))
        renderer.scale = 1
        let image = try #require(renderer.cgImage)
        #expect(image.width == 1920)
        // Both edges are backdrop; white stands in for the inset content.
        #expect(try pixel(image, x: 2, y: 500) == [255, 0, 0])
        #expect(try pixel(image, x: 1917, y: 500) == [255, 0, 0])
        #expect(try pixel(image, x: 103, y: 500) == [255, 0, 0])
        #expect(try pixel(image, x: 105, y: 500) == [255, 255, 255])
    }

    @Test @MainActor func largeContentCanExpandBeyondOrdinaryHeroHeight() throws {
        let renderer = ImageRenderer(content: TVMovieHeroLayout { white.frame(height: 800) }
            .background(red).frame(width: 1920))
        renderer.scale = 1
        let image = try #require(renderer.cgImage)
        #expect(image.height >= 880)
        #expect(try pixel(image, x: 120, y: 780) == [255, 255, 255])
    }

    private func pixel(_ image: CGImage, x: Int, y: Int) throws -> [UInt8] {
        let sample = try #require(image.cropping(to: CGRect(x: CGFloat(x), y: CGFloat(y), width: 1, height: 1)))
        let context = try #require(CGContext(data: nil, width: 1, height: 1, bitsPerComponent: 8, bytesPerRow: 4,
            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGBitmapInfo.byteOrder32Big.rawValue | CGImageAlphaInfo.premultipliedLast.rawValue))
        context.draw(sample, in: CGRect(x: 0, y: 0, width: 1, height: 1))
        let data = try #require(context.data?.assumingMemoryBound(to: UInt8.self))
        return [data[0], data[1], data[2]]
    }
}
#endif
