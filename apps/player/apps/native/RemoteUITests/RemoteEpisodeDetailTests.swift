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
        let plots = ["Dawn breaks over the river valley.", "The herd crosses the high pass.", "Rain settles over the forest.", "The journey ends at the coast."]
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label IN %@", plots)).firstMatch.exists, "The episode plot should show")
        XCTAssertTrue(app.buttons["show.season-1"].exists && app.buttons["show.season-2"].exists, "A show with two seasons should offer the season picker")
        record("episode detail", app)

        let play = app.buttons.matching(NSPredicate(format: "hasFocus == true AND (label BEGINSWITH 'Play' OR label BEGINSWITH 'Resume')")).firstMatch
        XCTAssertTrue(play.waitForExistence(timeout: 5), "Play should have the initial focus; focused: \(focusedLabels(app))")
        let row = play.frame.midY
        XCTAssertFalse(app.buttons["All episodes"].exists, "Episodes belong in the shelf below, not in a button")
        for button in [myList, app.buttons["Bookmarks"], app.buttons["More"]] {
            remote.press(.right)
            XCTAssertTrue(holdsFocus(button), "Right should reach \(button.label); focused: \(focusedLabels(app))")
            XCTAssertEqual(button.frame.midY, row, accuracy: 24, "\(button.label) should share the Play row")
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
