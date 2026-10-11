import SwiftUI
import XCTest
@testable import KinosailPlayer

@MainActor
final class MediaCardRatingTests: XCTestCase {
    // Failures: ratings remain visible, reserve a metadata row, or disappear from selected-video metadata.
    func testVideoCardsRenderIdenticallyWithAndWithoutRatings() throws {
        let session = AppSession()
        for kind in ["video", "show"] {
            for landscape in [false, true] {
                for size in [DynamicTypeSize.large, .accessibility3] {
                    let rated = try item(kind: kind, rating: "PG-13")
                    let unrated = try item(kind: kind, rating: "")
                    func render(_ item: MediaItem) throws -> CGImage {
                        try XCTUnwrap(ImageRenderer(content:
                            MediaCard(item: item, landscape: landscape)
                                .frame(width: landscape ? 390 : 230)
                                .environment(session).environment(\.dynamicTypeSize, size)
                                .environment(\.colorScheme, .dark)
                        ).cgImage)
                    }
                    let actual = try render(rated)
                    let expected = try render(unrated)
                    XCTAssertEqual(actual.width, expected.width)
                    XCTAssertEqual(actual.height, expected.height)
                    XCTAssertEqual(try pixels(actual), try pixels(expected), "\(kind), landscape=\(landscape), \(size)")
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

    // Compare rendered pixels directly. PNG compression is not part of the card
    // contract and can fail independently in a simulator's ImageIO encoder.
    private func pixels(_ image: CGImage) throws -> Data {
        let rowBytes = image.width * 4
        let context = try XCTUnwrap(CGContext(data: nil, width: image.width, height: image.height,
            bitsPerComponent: 8, bytesPerRow: rowBytes, space: CGColorSpaceCreateDeviceRGB(),
            bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.draw(image, in: CGRect(x: 0, y: 0, width: image.width, height: image.height))
        return Data(bytes: try XCTUnwrap(context.data), count: rowBytes * image.height)
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
