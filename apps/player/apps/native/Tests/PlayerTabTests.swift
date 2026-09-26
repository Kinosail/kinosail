import Testing
@testable import KinosailPlayer

struct PlayerTabTests {
    @Test func appleTVOpensEachMediaTypeFromTheTopMenu() {
        #expect(PlayerTab.tvPrimary == [.home, .movies, .shows, .music, .audiobooks, .photos, .library, .search, .settings])
        #expect(!PlayerTab.tvPrimary.contains(.more))
    }

    @Test func preservesPersonalOrder() throws {
        #expect(try PlayerTab.parse("shows,home,movies,list") == [.shows, .home, .movies, .list])
        #expect(PlayerTab.defaults == [.home, .shows, .movies, .search])
        #expect(try PlayerTab.parse("home,shows,movies,search") == PlayerTab.defaults)
        #expect(try PlayerTab.parse("home") == [.home])
    }
    @Test func upgradesOnlyLegacyDefaults() {
        #expect(PlayerTab.legacyDefault(nil) == "home,shows,movies,search")
        #expect(PlayerTab.legacyDefault("movies,shows") == "home,shows,movies,search")
        #expect(PlayerTab.legacyDefault("shows,movies,home") == "shows,movies,home")
        #expect(PlayerTab.legacyDefault("invalid") == "home,shows,movies,search")
    }
    @Test(arguments: ["", "movies,", ",movies", "movies,movies", "unknown", "more", " movies", "Movies", "movies,shows,home,list,library", String(repeating: "m", count: 129)])
    func rejectsInvalidPreferences(_ raw: String) {
        #expect(throws: ClientError.self) { try PlayerTab.parse(raw) }
    }

    @Test func keepsModeTabsIndependent() throws {
        #expect(PlayerMode.stored("listen") == .listen)
        #expect(PlayerMode.watch.defaultTabs == PlayerTab.defaults)
        #expect(PlayerMode.listen.defaultTabs == [.home, .music, .audiobooks, .search])
        #expect(try PlayerTab.parse(PlayerMode.listen.defaultTabs.map(\.rawValue).joined(separator: ",")) == PlayerMode.listen.defaultTabs)
        #expect(PlayerMode.watch.tabsKey("viewer") == "kinosail.tabs.v2.viewer")
        #expect(PlayerMode.listen.tabsKey("viewer") != PlayerMode.watch.tabsKey("viewer"))
        #expect(PlayerMode.watch.searchViews == [.movies, .shows])
        #expect(PlayerMode.listen.searchViews == [.music, .audiobooks])
    }

    @Test(arguments: [nil, "", "WATCH", "music", String(repeating: "x", count: 129)] as [String?])
    func invalidStoredModeFallsBackWithoutChangingWatchTabs(_ raw: String?) {
        #expect(PlayerMode.stored(raw) == .watch)
        #expect(PlayerMode.watch.tabsKey("viewer") == "kinosail.tabs.v2.viewer")
    }
}
