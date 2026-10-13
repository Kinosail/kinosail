import SwiftUI
import XCTest
@testable import KinosailPlayer

@MainActor
final class MediaCardRatingTests: XCTestCase {
    // E2E does not exercise this raster helper. It must not omit text and compare backgrounds alone.
    func testCardCaptureIncludesVisibleRatingText() throws {
        let session = AppSession()
        let rated = try item(kind: "video", rating: "PG-13")
        func render(opacity: Double) throws -> (image: CGImage, pixels: Data) {
            try capture(MediaCard(item: rated, landscape: false)
                .frame(width: 230).environment(session)
                .environment(\.dynamicTypeSize, .large).environment(\.colorScheme, .dark)
                .overlay(alignment: .topLeading) {
                    Text("PG-13").font(.headline).foregroundStyle(.white).padding(8).opacity(opacity)
                })
        }
        let hidden = try render(opacity: 0), visible = try render(opacity: 1)
        XCTAssertEqual(hidden.image.width, visible.image.width)
        XCTAssertEqual(hidden.image.height, visible.image.height)
        XCTAssertNotEqual(hidden.pixels, visible.pixels, "Visible rating text must change the captured pixels")
        for (label, raster) in [("hidden", hidden), ("visible", visible)] {
            XCTAssertTrue(stride(from: 3, to: raster.pixels.count, by: 4).contains { raster.pixels[$0] != 0 })
            let attachment = XCTAttachment(image: UIImage(cgImage: raster.image))
            attachment.name = "rating-text-control-\(label)"
            attachment.lifetime = .keepAlways
            add(attachment)
        }
    }

    // Failures: ratings remain visible, reserve a metadata row, or disappear from selected-video metadata.
    func testVideoCardsRenderIdenticallyWithAndWithoutRatings() throws {
        let session = AppSession()
        for kind in ["video", "show"] {
            for landscape in [false, true] {
                for size in [DynamicTypeSize.large, .accessibility3] {
                    let rated = try item(kind: kind, rating: "PG-13")
                    let unrated = try item(kind: kind, rating: "")
                    func render(_ item: MediaItem) throws -> (image: CGImage, pixels: Data) {
                        try capture(
                            MediaCard(item: item, landscape: landscape)
                                .frame(width: landscape ? 390 : 230)
                                .environment(session).environment(\.dynamicTypeSize, size)
                                .environment(\.colorScheme, .dark)
                        )
                    }
                    let (actual, actualPixels) = try render(rated)
                    let (expected, expectedPixels) = try render(unrated)
                    let actualHasPixels = stride(from: 3, to: actualPixels.count, by: 4).contains { actualPixels[$0] != 0 }
                    let expectedHasPixels = stride(from: 3, to: expectedPixels.count, by: 4).contains { expectedPixels[$0] != 0 }
                    if actual.width != expected.width || actual.height != expected.height || actualPixels != expectedPixels || !actualHasPixels || !expectedHasPixels {
                        for (label, image, rgba) in [("rated", actual, actualPixels), ("unrated", expected, expectedPixels)] {
                            let name = "\(kind)-\(landscape)-\(size)-\(label)-\(image.width)x\(image.height)"
                            let rendered = XCTAttachment(image: UIImage(cgImage: image))
                            rendered.name = "pair-\(name)"
                            rendered.lifetime = .keepAlways
                            add(rendered)
                            let raw = XCTAttachment(data: rgba, uniformTypeIdentifier: "public.data")
                            raw.name = "rgba-\(name)"
                            raw.lifetime = .keepAlways
                            add(raw)
                        }
                    }
                    XCTAssertEqual(actual.width, expected.width)
                    XCTAssertEqual(actual.height, expected.height)
                    XCTAssertTrue(actualHasPixels, "Rated card capture must contain visible pixels")
                    XCTAssertTrue(expectedHasPixels, "Unrated card capture must contain visible pixels")
                    XCTAssertEqual(actualPixels, expectedPixels, "\(kind), landscape=\(landscape), \(size)")
                    XCTAssertEqual(rated.rating, "PG-13")
                    XCTAssertTrue(rated.subtitle.contains("PG-13"))
                    let attachment = XCTAttachment(image: UIImage(cgImage: actual))
                    attachment.name = "\(kind)-\(landscape ? "landscape" : "poster")-\(size)"
                    attachment.lifetime = .keepAlways
                    add(attachment)
                }
            }
        }
    }

    // Failures: blank capture, cropped content, or attachments from a different raster.
    // Draw directly into the same bitmap used for assertions and image evidence.
    private func capture<Content: View>(_ content: Content) throws -> (image: CGImage, pixels: Data) {
        let renderer = ImageRenderer(content: content)
        var result: Result<(image: CGImage, pixels: Data), Error>?
        renderer.render(rasterizationScale: 1) { size, draw in
            result = Result {
                let bounds = try XCTUnwrap(size.width.isFinite && size.height.isFinite &&
                    (1...4096).contains(size.width) && (1...4096).contains(size.height) ? size : nil,
                    "Card capture must have finite positive dimensions within the fixture limit")
                let width = Int(ceil(bounds.width)), height = Int(ceil(bounds.height))
                let rowBytes = width * 4
                let context = try XCTUnwrap(CGContext(data: nil, width: width, height: height,
                    bitsPerComponent: 8, bytesPerRow: rowBytes, space: CGColorSpaceCreateDeviceRGB(),
                    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
                context.clear(CGRect(x: 0, y: 0, width: width, height: height))
                draw(context)
                return try withExtendedLifetime(context) {
                    let pixels = Data(bytes: try XCTUnwrap(context.data), count: rowBytes * height)
                    return (try XCTUnwrap(context.makeImage()), pixels)
                }
            }
        }
        return try XCTUnwrap(result, "ImageRenderer must invoke the capture closure").get()
    }

    private func item(kind: String, rating: String) throws -> MediaItem {
        try MediaItem(.object([
            "id": .string("sample"), "kind": .string(kind),
            "title": .string("A movie title that wraps onto two lines"),
            "year": .string("2024"), "rating": .string(rating),
            "progress": .object(["seconds": .number(44)])
        ]), server: ServerAddress("https://media.example"))
    }
}
