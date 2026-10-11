#if os(tvOS)
import SwiftUI
import Testing
@testable import KinosailPlayer

struct TVMovieDetailTests {
    @Test func usesBackdropOnlyForMoviesWithArtwork() throws {
        let server = try ServerAddress("https://media.example")
        func item(_ fields: [String: JSONValue]) throws -> MediaItem {
            try MediaItem(.object(["id": .string("movie"), "kind": .string("video"),
                                   "title": .string("Movie"), "backdrop": .string("/backdrop/movie")]
                .merging(fields) { _, new in new }), server: server)
        }
        #expect(try TVMovieDetail(item: item([:])).usesBackdrop)
        #expect(try !TVMovieDetail(item: item(["backdrop": .string("")])).usesBackdrop)
        #expect(try !TVMovieDetail(item: item(["show": .string("Series")])).usesBackdrop)
        #expect(try !TVMovieDetail(item: item(["showId": .string("0123456789abcdef")])).usesBackdrop)
        #expect(try !TVMovieDetail(item: item(["kind": .string("music")])).usesBackdrop)
    }

    @Test func metadataUsesOnlyAvailableMovieFacts() throws {
        let server = try ServerAddress("https://media.example")
        let full = try MediaItem(.object(["id": .string("movie"), "kind": .string("video"),
                                          "title": .string("Movie"), "year": .string("2004"),
                                          "rating": .string("R"), "genres": .string("Action, Adventure")]), server: server)
        #expect(TVMovieDetail(item: full).metadata == "2004 · R · Action, Adventure")
        let sparse = try MediaItem(.object(["id": .string("movie"), "kind": .string("video"),
                                            "title": .string("Movie"), "genres": .string("Drama")]), server: server)
        #expect(TVMovieDetail(item: sparse).metadata == "Drama")
    }

    @Test @MainActor func pendingDetailReservesHeroAndCastSpace() {
        let image = ImageRenderer(content: DetailLoadingState().frame(width: 1792)).uiImage
        #expect((image?.size.height ?? 0) >= 650)
    }

    // Remote E2E owns lifecycle/focus and actual screen proof. Its loading
    // placeholders are AX-hidden, so this painted check protects card pitch.
    @Test @MainActor func pendingCastCardsMatchPopulatedShelfPitch() throws {
        let renderer = ImageRenderer(content: DetailLoadingState().frame(width: 1792)
            .environment(\.colorScheme, .dark))
        renderer.scale = 1
        let spans = try pendingCardSpans(#require(renderer.cgImage))
        #expect(spans.count == 4)
        for span in spans { #expect(span.width == 230) }
        for pair in zip(spans, spans.dropFirst()) {
            #expect(pair.1.start - pair.0.start == 248)
        }
    }

    private func pendingCardSpans(_ image: CGImage) throws -> [(start: Int, width: Int)] {
        // Inside the cards, below their rounded corners. The gaps stay clear.
        let row = try #require(image.cropping(to: CGRect(x: 0, y: CGFloat(image.height - 100),
            width: CGFloat(image.width), height: 1)))
        let context = try #require(CGContext(data: nil, width: image.width, height: 1,
            bitsPerComponent: 8, bytesPerRow: image.width * 4,
            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGBitmapInfo.byteOrder32Big.rawValue
                | CGImageAlphaInfo.premultipliedLast.rawValue))
        context.draw(row, in: CGRect(x: 0, y: 0, width: CGFloat(image.width), height: 1))
        let data = try #require(context.data?.assumingMemoryBound(to: UInt8.self))
        var spans: [(start: Int, width: Int)] = []
        var start: Int?
        for x in 0...image.width {
            let opaque = x < image.width && data[x * 4 + 3] > 127
            if opaque && start == nil { start = x }
            if !opaque, let first = start {
                spans.append((first, x - first))
                start = nil
            }
        }
        return spans
    }
}
#endif
