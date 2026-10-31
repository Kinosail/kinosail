import XCTest

final class RemoteEpisodeDetailTests: RemoteTestCase {
    @MainActor
    func testEpisodeDetailKeepsActionsInOneRowAboveTheEpisodes() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        remote.press(.down)
        remote.press(.down)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Shows"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS 'Into the Wild'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 10))
        if !card.hasFocus { remote.press(.down) }
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Episodes"].waitForExistence(timeout: 10))
        remote.press(.down)
        remote.press(.down)
        let episode = focusedLabels(app).first ?? ""
        XCTAssertFalse(episode.isEmpty, "Down should reach an episode")
        remote.press(.select, forDuration: 1.2)
        XCTAssertTrue(app.descendants(matching: .any)["Details"].waitForExistence(timeout: 5), "Press and hold should offer Details")
        remote.press(.select)
        let myList = app.buttons.matching(NSPredicate(format: "label == 'My List' OR label == 'In My List'")).firstMatch
        XCTAssertTrue(myList.waitForExistence(timeout: 10), app.debugDescription)
        record("episode detail", app)

        let play = app.buttons.matching(NSPredicate(format: "hasFocus == true AND (label BEGINSWITH 'Play' OR label BEGINSWITH 'Resume')")).firstMatch
        XCTAssertTrue(play.waitForExistence(timeout: 5), "Play should have the initial focus; focused: \(focusedLabels(app))")
        let row = play.frame.midY
        XCTAssertFalse(app.buttons["All episodes"].exists, "Episodes belong in the shelf below, not in a button")
        for expected in ["My List", "Bookmarks", "More"] {
            remote.press(.right)
            let focused = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
            XCTAssertTrue(focused.label.contains(expected) || expected == "My List" && focused.label == "In My List",
                          "Right should reach \(expected); focused: \(focusedLabels(app))")
            XCTAssertEqual(focused.frame.midY, row, accuracy: 24, "\(expected) should share the Play row")
        }
        remote.press(.down)
        XCTAssertTrue(app.staticTexts["Episodes"].exists)
        XCTAssertTrue(focusedLabels(app).contains(episode), "Down should land on the current episode; focused: \(focusedLabels(app))")
        record("episode detail episodes shelf", app)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Episodes"].waitForExistence(timeout: 10))
        XCTAssertTrue(focusedLabels(app).contains(episode), "Back should restore the episode on the show page")
    }
}
