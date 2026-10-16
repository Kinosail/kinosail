#if os(tvOS)
import SwiftUI
import Testing
@testable import KinosailPlayer

@Suite(.serialized) struct TVMovieDetailTests {
    @Test func usesBackdropForVideosWithArtwork() throws {
        let server = try ServerAddress("https://media.example")
        func item(_ fields: [String: JSONValue]) throws -> MediaItem {
            try MediaItem(.object(["id": .string("movie"), "kind": .string("video"),
                                   "title": .string("Movie"), "backdrop": .string("/backdrop/movie")]
                .merging(fields) { _, new in new }), server: server)
        }
        #expect(try TVMovieDetail(item: item([:])).usesBackdrop)
        #expect(try !TVMovieDetail(item: item(["backdrop": .string("")])).usesBackdrop)
        #expect(try TVMovieDetail(item: item(["show": .string("Series"), "showId": .string("0123456789abcdef")])).usesBackdrop)
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
        let episode = try MediaItem(.object(["id": .string("episode"), "kind": .string("video"),
                                             "title": .string("S01E02 · Pilot"), "show": .string("Series"),
                                             "showId": .string("0123456789abcdef"), "season": .number(1),
                                             "episode": .number(2), "rating": .string("PG")]), server: server)
        #expect(TVMovieDetail(item: episode).metadata == "Series · S1 E2 · PG")
        #expect(TVMovieDetail(item: episode).title == "Pilot")
    }

    @Test @MainActor func pendingDetailReservesHeroAndCastSpace() {
        let image = ImageRenderer(content: DetailLoadingState().frame(width: 1792)).uiImage
        #expect((image?.size.height ?? 0) >= 650)
    }

    @Test @MainActor func pendingCastCardsMatchPopulatedShelfPitch() async throws {
        let scene = try #require(UIApplication.shared.connectedScenes.first as? UIWindowScene)
        let window = UIWindow(windowScene: scene)
        let controller = UIHostingController(rootView: DetailLoadingState()
            .frame(width: 1792).environment(\.colorScheme, .dark))
        controller.view.backgroundColor = .clear
        controller.view.isOpaque = false
        window.rootViewController = controller
        window.isHidden = false
        defer { window.isHidden = true }
        let size = controller.sizeThatFits(in: CGSize(width: 1792, height: 2000))
        controller.view.frame = CGRect(origin: .zero, size: size)
        controller.view.setNeedsLayout()
        controller.view.layoutIfNeeded()
        try await Task.sleep(for: .milliseconds(100))
        let format = UIGraphicsImageRendererFormat()
        format.scale = 1
        format.opaque = false
        let painted = UIGraphicsImageRenderer(size: size, format: format).image { _ in
            controller.view.drawHierarchy(in: controller.view.bounds, afterScreenUpdates: true)
        }
        let spans = try pendingCardSpans(#require(painted.cgImage))
        #expect(spans.count == 6)
        for span in spans { #expect(span.width == 230) }
        for pair in zip(spans, spans.dropFirst()) {
            #expect(pair.1.start - pair.0.start == 248)
        }
    }

    private func pendingCardSpans(_ image: CGImage) throws -> [(start: Int, width: Int)] {
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
