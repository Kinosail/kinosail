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
}
#endif
