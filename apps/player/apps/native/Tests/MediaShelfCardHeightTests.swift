import SwiftUI
import Testing
@testable import KinosailPlayer

@MainActor
struct MediaShelfCardHeightTests {
    private let session = AppSession()

    @Test(arguments: [true, false])
    func episodeAndMovieCardsShareOneHeightInAShelf(resumes: Bool) throws {
        let items = [try movie(), try episode()]
        let reserves = MediaShelf.reservesSubtitle(for: items, resumesPlayback: resumes, opensShows: false)
        let heights = items.map { height(MediaCard(item: $0, landscape: true, resumesPlayback: resumes, reservesSubtitle: reserves)) }
        #expect(heights[0] > 0)
        #expect(heights[0] == heights[1])
    }

    @Test func movieShelvesKeepTheirCompactHeight() throws {
        let items = [try movie(), try movie()]
        let reserves = MediaShelf.reservesSubtitle(for: items, resumesPlayback: true, opensShows: false)
        #expect(!reserves)
        #expect(height(MediaCard(item: items[0], landscape: true, resumesPlayback: true, reservesSubtitle: reserves))
                == height(MediaCard(item: items[0], landscape: true, resumesPlayback: true)))
    }

    private func height(_ card: MediaCard) -> CGFloat {
        let width: CGFloat = 390
        let host = UIHostingController(rootView: card.frame(width: width).environment(session)
            .environment(\.dynamicTypeSize, .large).environment(\.colorScheme, .dark))
        return host.sizeThatFits(in: CGSize(width: width, height: .greatestFiniteMagnitude)).height
    }

    private func movie() throws -> MediaItem {
        try MediaItem(.object([
            "id": .string("movie"), "kind": .string("video"), "title": .string("Toy Story"),
            "year": .string("1995"), "progress": .object(["seconds": .number(600)])
        ]), server: ServerAddress("https://media.example"))
    }

    private func episode() throws -> MediaItem {
        try MediaItem(.object([
            "id": .string("episode"), "kind": .string("video"), "title": .string("Pilot"),
            "show": .string("Example Show"), "showId": .string("0123456789abcdef"), "season": .number(1), "episode": .number(1),
            "progress": .object(["seconds": .number(300)])
        ]), server: ServerAddress("https://media.example"))
    }
}
