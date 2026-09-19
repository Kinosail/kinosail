import Foundation
import Testing
@testable import KinosailPlayer

struct MediaLinkTests {
    private let scope = String(repeating: "a", count: 64)

    @Test(arguments: [MediaLink.Action.play, .detail, .search])
    func roundTrip(action: MediaLink.Action) throws {
        let link = try MediaLink(action: action, value: action == .search ? "Wallace & Gromit" : "movie_1", scope: scope)
        #expect(try MediaLink(url: link.url) == link)
        #expect(link.url.user == nil)
        #expect(link.url.password == nil)
    }

    @Test(arguments: ["", " ", "../movie", "movie/file", "movie?token=x", "movie\n", String(repeating: "a", count: 129)])
    func rejectsInvalidTitleID(value: String) {
        #expect(throws: (any Error).self) { try MediaLink(action: .play, value: value, scope: scope) }
    }

    @Test(arguments: ["", " ", "word\nword", String(repeating: "x", count: 513)])
    func rejectsInvalidSearch(value: String) {
        #expect(throws: (any Error).self) { try MediaLink(action: .search, value: value, scope: scope) }
    }

    @Test(arguments: ["", "abc", String(repeating: "A", count: 64), String(repeating: "g", count: 64)])
    func rejectsInvalidScope(value: String) {
        #expect(throws: (any Error).self) { try MediaLink(action: .play, value: "movie", scope: value) }
    }

    @Test(arguments: [
        "https://media?", "kinosail://other?", "kinosail://media/path?",
        "kinosail://user@media?", "kinosail://media:42?"
    ])
    func rejectsForeignDestinations(prefix: String) throws {
        let url = try #require(URL(string: prefix + "action=play&value=movie&scope=" + scope))
        #expect(throws: (any Error).self) { try MediaLink(url: url) }
    }

    @Test(arguments: [
        "action=play&value=movie", "action=unknown&value=movie&scope=", "action=play&value&scope=",
        "action=play&value=movie&extra=x&scope=", "action=play&action=detail&value=movie&scope=",
        "action=play&value=movie&scope=bad&scope=", "action=play&value=movie&scope="
    ])
    @MainActor func rejectsMalformedLinksWithoutStartingPlayback(query: String) throws {
        let session = AppSession()
        // The final variant is syntactically valid but belongs to no active session.
        session.handleIncomingURL(try #require(URL(string: "kinosail://media?" + query + scope)))
        #expect(session.pendingMediaLink == nil)
        #expect(session.player.currentItem == nil)
        #expect(session.client == nil)
        #expect(session.notice != nil)
    }

    @Test func rejectsFragmentAndOversizedURL() throws {
        let url = try MediaLink(action: .play, value: "movie", scope: scope).url
        for suffix in ["#fragment", "&extra=" + String(repeating: "x", count: 4096)] {
            #expect(throws: (any Error).self) { try MediaLink(url: #require(URL(string: url.absoluteString + suffix))) }
        }
    }
}
