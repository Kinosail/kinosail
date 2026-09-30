#if os(tvOS)
import SwiftUI
import Testing
import UIKit
@testable import KinosailPlayer

// Failure cases: a second sail in the brand column breaks the page canvas;
// an opaque fill hides the shared sail; light appearance must omit decoration.
@MainActor struct TVOSBackgroundTests {
    @Test(arguments: [ColorScheme.dark, .light])
    func configurationUsesOneSharedCanvas(_ appearance: ColorScheme) throws {
        let configuration = TVOSConfigurationLayout(title: "Settings", symbol: "gearshape") {
            Text("Settings row").frame(width: 500, height: 200)
        }
        let actual = try render(configuration, appearance: appearance)
        let expected = try render(Color.clear.cinemaBackground(), appearance: appearance)
        for point in [CGPoint(x: 680, y: 160), CGPoint(x: 1850, y: 220)] {
            #expect(try pixel(actual, at: point) == pixel(expected, at: point),
                    "Brand and page must use the same background in every appearance")
        }
    }

    private func render(_ view: some View, appearance: ColorScheme) throws -> CGImage {
        let renderer = ImageRenderer(content: view.frame(width: 1920, height: 1080)
            .environment(\.colorScheme, appearance))
        renderer.scale = 1
        return try #require(renderer.uiImage?.cgImage)
    }

    private func pixel(_ image: CGImage, at point: CGPoint) throws -> [UInt8] {
        let sample = try #require(image.cropping(to: CGRect(origin: point, size: CGSize(width: 1, height: 1))))
        var color = [UInt8](repeating: 0, count: 4)
        color.withUnsafeMutableBytes { bytes in
            let context = CGContext(data: bytes.baseAddress, width: 1, height: 1, bitsPerComponent: 8,
                                    bytesPerRow: 4, space: CGColorSpaceCreateDeviceRGB(),
                                    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
            context.draw(sample, in: CGRect(x: 0, y: 0, width: 1, height: 1))
        }
        return color
    }
}
#endif
