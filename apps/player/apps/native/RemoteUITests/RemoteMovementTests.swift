import XCTest

final class RemoteMovementTests: RemoteTestCase {
    @MainActor
    func testLaunchAndLibraryStartOnTopLeftTitle() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15), app.debugDescription)
        let firstCard = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
        XCTAssertTrue(firstCard.waitForExistence(timeout: 5))
        let launchFocus = focusedLabels(app)
        record("launch focus", app)
        XCTAssertFalse(app.buttons["Search"].hasFocus, "Launch should not focus Search")
        XCTAssertFalse(app.buttons["Settings"].hasFocus, "Launch should not focus Settings")
        remote.press(.left)
        XCTAssertEqual(focusedLabels(app), launchFocus, "Launch should focus the leftmost Continue watching title")
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus, "Down from the first title should reach Browse")
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10), app.debugDescription)
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS 'After the Rain'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 10))
        let focused = expectation(for: NSPredicate(format: "hasFocus == true"), evaluatedWith: card)
        wait(for: [focused], timeout: 5)
        record("movies focus", app)
        XCTAssertFalse(focusedLabels(app).contains(where: { $0.hasPrefix("Sort:") }), "Movies should not open on Sort")
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Movies"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.up)
        XCTAssertEqual(focusedLabels(app), launchFocus)
        remote.press(.up)
        XCTAssertTrue(app.buttons["Search"].hasFocus)
    }

    @MainActor
    func testTopRailFarEdgeUpReachesSettingsAboveIt() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        remote.press(.down)
        for _ in 0..<3 { remote.press(.right) }
        record("fourth watching card", app)
        remote.press(.up)
        XCTAssertTrue(app.buttons["Settings"].hasFocus,
                      "Up from the rightmost visible card should reach Settings directly above it, not jump across to Search")
        record("up from fourth watching card", app)
        remote.press(.down)
        record("down from Settings to watching", app)
        XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains("Moonrise") }),
                      "Down should return to the card below Settings")
        remote.press(.up)
        XCTAssertTrue(app.buttons["Settings"].hasFocus)
    }

    @MainActor
    func testTopBarAndBrowseReturnPaths() {
        let remote = XCUIRemote.shared
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        launchAtSearch(app)
        let search = app.buttons["Search"]
        let settings = app.buttons["Settings"]
        XCTAssertTrue(search.waitForExistence(timeout: 15), app.debugDescription)
        XCTAssertTrue(search.hasFocus)
        XCTAssertFalse(app.buttons["Library"].exists, "Browse should offer the media categories directly")
        record("launch", app)

        remote.press(.right)
        XCTAssertTrue(settings.hasFocus)
        record("right", app)
        remote.press(.left)
        XCTAssertTrue(search.hasFocus)
        record("left", app)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10), app.debugDescription)
        record("search", app)
        remote.press(.menu)
        XCTAssertTrue(search.waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(search.hasFocus)
        record("back from search", app)

        remote.press(.right)
        XCTAssertTrue(settings.hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Playback"].waitForExistence(timeout: 10), app.debugDescription)
        record("settings", app)
        remote.press(.menu)
        XCTAssertTrue(settings.waitForExistence(timeout: 10), app.debugDescription)
        record("back from settings", app)

        remote.press(.left)
        remote.press(.down)
        record("down from search", app)
        remote.press(.down)
        record("down again", app)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.right)
        record("browse right", app)
        XCTAssertTrue(app.buttons["Shows"].hasFocus)
        remote.press(.left)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10), app.debugDescription)
        record("movies", app)
        remote.press(.down)
        record("movies down", app)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Movies"].waitForExistence(timeout: 10), app.debugDescription)
        record("back from movies", app)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)

        for title in ["Shows", "Music", "Audiobooks", "Photos"] {
            remote.press(.right)
            XCTAssertTrue(app.buttons[title].hasFocus, "Right should focus \(title)")
            record("focus \(title)", app)
            remote.press(.select)
            XCTAssertTrue(app.staticTexts[title == "Music" ? "Albums" : title]
                .waitForExistence(timeout: 10), app.debugDescription)
            record("open \(title)", app)
            remote.press(.menu)
            XCTAssertTrue(app.buttons[title].waitForExistence(timeout: 10), app.debugDescription)
            XCTAssertTrue(app.buttons[title].hasFocus, "Back should restore \(title)")
            record("back from \(title)", app)
        }
        remote.press(.right)
        XCTAssertTrue(app.buttons["Photos"].hasFocus, "Right should stop at the last Browse tile")
        XCTAssertFalse(app.buttons["Library"].exists)
        remote.press(.left)
        XCTAssertTrue(app.buttons["Audiobooks"].hasFocus)
    }

    @MainActor
    func testMovieDetailBackRestoresGridAndBrowseFocus() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15), app.debugDescription)
        remote.press(.down)
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10), app.debugDescription)
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS 'After the Rain'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 10), "Movies should load a card")
        if !card.hasFocus { remote.press(.down) }
        XCTAssertTrue(card.hasFocus)
        let title = card.staticTexts.firstMatch.label
        XCTAssertFalse(title.isEmpty)
        record("movie grid card", app)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(app.staticTexts[title].exists)
        record("movie detail", app)
        remote.press(.down)
        record("movie down to secondary row", app)
        XCTAssertTrue(app.buttons["My List"].hasFocus, "Down should reach the secondary action below Resume")
        remote.press(.right)
        XCTAssertTrue(app.buttons["Bookmarks"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["No bookmarks yet"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.descendants(matching: .any).matching(NSPredicate(
            format: "hasFocus == true AND label == 'Back'"
        )).firstMatch.waitForExistence(timeout: 5))
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Bookmarks"].hasFocus, "Back should restore the action that opened Bookmarks")
        remote.press(.right)
        XCTAssertTrue(app.buttons["Remove from Continue watching"].hasFocus)
        record("movie rightmost secondary action", app)
        remote.press(.up)
        XCTAssertTrue(app.buttons["Resume"].hasFocus,
                      "Up should reach the only primary action above the secondary row in one press")
        record("movie secondary row up to Resume", app)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10), app.debugDescription)
        record("back to movie grid", app)
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "label CONTAINS %@", title)).firstMatch.hasFocus,
                      "Back should restore the selected movie")
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Movies"].waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        record("back to browse", app)
    }

    @MainActor
    func testPlaybackMenuReturnsToMovieDetail() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15), app.debugDescription)
        remote.press(.down)
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10), app.debugDescription)
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS 'After the Rain'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 10))
        if !card.hasFocus { remote.press(.down) }
        XCTAssertTrue(card.hasFocus)
        remote.press(.select)
        let bookmarks = app.buttons["Bookmarks"]
        XCTAssertTrue(bookmarks.waitForExistence(timeout: 10), app.debugDescription)
        record("before playback", app)
        remote.press(.select)
        XCTAssertTrue(bookmarks.waitForNonExistence(timeout: 15), "Select should open full-screen playback")
        XCTAssertTrue(app.cells["Playback options"].waitForExistence(timeout: 20), "Video controls should load")
        record("playback loaded", app)
        remote.press(.menu)
        XCTAssertTrue(bookmarks.waitForExistence(timeout: 10), "Back should return to the movie detail")
        record("back from playback", app)
    }

    @MainActor
    func testPhotoZoomAndBackRestoresBrowseFocus() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15), app.debugDescription)
        remote.press(.down)
        remote.press(.down)
        for title in ["Shows", "Music", "Audiobooks", "Photos"] {
            remote.press(.right)
            XCTAssertTrue(app.buttons[title].hasFocus)
        }
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Photos"].waitForExistence(timeout: 10), app.debugDescription)
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS 'Color Study'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 10), "Photos should load a card")
        if !card.hasFocus { remote.press(.down) }
        XCTAssertTrue(card.hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Select to zoom"].waitForExistence(timeout: 10), app.debugDescription)
        record("photo viewer", app)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Select to fit · Move to pan"].waitForExistence(timeout: 5))
        remote.press(.right)
        record("photo panned", app)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Select to zoom"].waitForExistence(timeout: 5))
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "label CONTAINS 'Color Study'")).firstMatch.hasFocus)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Photos"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Photos"].hasFocus)
        record("back to Photos tile", app)
    }

    @MainActor
    func testShowSeasonsAndBackRestoreFocus() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Shows"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS 'Into the Wild'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 10))
        if !card.hasFocus { remote.press(.down) }
        XCTAssertTrue(card.hasFocus)
        let title = card.staticTexts.firstMatch.label
        remote.press(.select)
        XCTAssertTrue(app.staticTexts[title].waitForExistence(timeout: 10))
        XCTAssertTrue(app.staticTexts["Episodes"].waitForExistence(timeout: 10))
        record("show detail", app)
        remote.press(.down)
        let season = focusedLabels(app).first ?? ""
        XCTAssertTrue(season.hasPrefix("Season") || season == "Specials", "Down should reach a season")
        record("show season", app)
        remote.press(.down)
        XCTAssertNotEqual(focusedLabels(app).first, season, "Down should reach an episode")
        record("show episode", app)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "label CONTAINS %@", title)).firstMatch.hasFocus)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Shows"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Shows"].hasFocus)
        record("back to Shows tile", app)
    }

}
