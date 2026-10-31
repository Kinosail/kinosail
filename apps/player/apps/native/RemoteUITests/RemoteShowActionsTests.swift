import XCTest

final class RemoteShowActionsTests: RemoteTestCase {
    @MainActor
    func testShowAndEpisodeActionsMarkWatchedAndSaveTheShow() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        openShow(app)

        let play = app.buttons.matching(NSPredicate(format: "hasFocus == true AND (label BEGINSWITH 'Play' OR label BEGINSWITH 'Resume')")).firstMatch
        XCTAssertTrue(play.waitForExistence(timeout: 10), "Play should have the initial focus; focused: \(focusedLabels(app))")
        let row = play.frame.midY
        let myList = app.buttons["My List"]
        XCTAssertTrue(myList.waitForExistence(timeout: 10), app.debugDescription)
        for button in [myList, app.buttons["More"]] {
            remote.press(.right)
            XCTAssertTrue(holdsFocus(button), "Right should reach \(button.label); focused: \(focusedLabels(app))")
            XCTAssertEqual(button.frame.midY, row, accuracy: 24, "\(button.label) should share the Play row")
        }
        record("show actions", app)

        remote.press(.left)
        remote.press(.select)
        XCTAssertTrue(app.buttons["In My List"].waitForExistence(timeout: 10), "My List should save the show")
        remote.press(.right)
        choose("Mark show as watched", in: app)
        settle()
        remote.press(.select)
        XCTAssertTrue(menuItem("Mark show as unwatched", in: app).waitForExistence(timeout: 10), "The show should read as watched; \(app.debugDescription)")
        record("show more menu after mark watched", app)
        remote.press(.menu)

        remote.press(.down)
        remote.press(.down)
        remote.press(.select, forDuration: 1.2)
        XCTAssertTrue(app.descendants(matching: .any)["Details"].waitForExistence(timeout: 5))
        remote.press(.select)
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "label == 'My List' OR label == 'In My List'")).firstMatch.waitForExistence(timeout: 10))
        for _ in 0..<3 { remote.press(.right) }
        XCTAssertTrue(holdsFocus(app.buttons["More"]), "focused: \(focusedLabels(app))")
        choose("Mark as unwatched", in: app)
        settle()
        remote.press(.select)
        XCTAssertTrue(menuItem("Mark as watched", in: app).waitForExistence(timeout: 10), "The episode should read as unwatched")
        XCTAssertTrue(app.descendants(matching: .any).matching(NSPredicate(format: "label BEGINSWITH %@", "Mark season")).firstMatch.exists)
        record("episode more menu", app)
        remote.press(.menu)
        remote.press(.menu)

        let saved = app.buttons["In My List"], more = app.buttons["More"]
        XCTAssertTrue(saved.waitForExistence(timeout: 10))
        for _ in 0..<4 where !holdsFocus(saved) && !holdsFocus(more) && !focusedLabels(app).contains(where: { $0.hasPrefix("Play") || $0.hasPrefix("Resume") }) { remote.press(.up) }
        for _ in 0..<2 where !holdsFocus(saved) { remote.press(holdsFocus(more) ? .left : .right) }
        XCTAssertTrue(holdsFocus(saved), "focused: \(focusedLabels(app))")
        remote.press(.select)
        XCTAssertTrue(app.buttons["My List"].waitForExistence(timeout: 10), "My List should remove the show")
        remote.press(.right)
        choose("Mark show as unwatched", in: app)
    }

    @MainActor
    private func openShow(_ app: XCUIApplication) {
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        remote.press(.down)
        remote.press(.down)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Shows"].hasFocus)
        remote.press(.select)
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS 'Into the Wild'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 10))
        if !card.hasFocus { remote.press(.down) }
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Episodes"].waitForExistence(timeout: 10))
    }

    @MainActor
    private func choose(_ label: String, in app: XCUIApplication) {
        let remote = XCUIRemote.shared
        remote.press(.select)
        let item = menuItem(label, in: app)
        XCTAssertTrue(item.waitForExistence(timeout: 5), "More should offer \(label); \(app.debugDescription)")
        let current = app.descendants(matching: .any).matching(NSPredicate(format: "label == %@ AND (hasFocus == true OR selected == true)", label)).firstMatch
        for _ in 0..<6 where !current.exists { remote.press(.down) }
        XCTAssertTrue(current.exists, "\(label) should take focus; \(app.debugDescription)")
        remote.press(.select)
    }
}

extension RemoteShowActionsTests {
    @MainActor
    func menuItem(_ label: String, in app: XCUIApplication) -> XCUIElement {
        app.descendants(matching: .any).matching(NSPredicate(format: "label == %@", label)).firstMatch
    }
}

extension RemoteShowActionsTests {
    @MainActor
    func settle() {
        RunLoop.current.run(until: Date().addingTimeInterval(2))
    }
}
