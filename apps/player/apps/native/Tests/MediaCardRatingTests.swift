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
                    func render(_ item: MediaItem) throws -> UIImage {
                        try XCTUnwrap(ImageRenderer(content:
                            MediaCard(item: item, landscape: landscape)
                                .frame(width: landscape ? 390 : 230)
                                .environment(session).environment(\.dynamicTypeSize, size)
                                .environment(\.colorScheme, .dark)
                        ).uiImage)
                    }
                    let actual = try render(rated)
                    let expected = try render(unrated)
                    XCTAssertEqual(actual.pngData(), expected.pngData(), "\(kind), landscape=\(landscape), \(size)")
                    XCTAssertEqual(rated.rating, "PG-13")
                    XCTAssertTrue(rated.subtitle.contains("PG-13"))
                    let attachment = XCTAttachment(image: actual)
                    attachment.name = "\(kind)-\(landscape ? "landscape" : "poster")-\(size)"
                    attachment.lifetime = .keepAlways
                    add(attachment)
                }
            }
        }
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
