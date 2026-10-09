import XCTest

// Uses the populated loopback QA session and its four genre shelves.
final class RemoteHomeScrollingTests: RemoteTestCase {
    @MainActor
    func testDeepHomeTraversalPreservesFocusAndReturnsToSearch() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        var downMovements = 1
        app.launch()
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        XCTAssertTrue(app.buttons["Search"].hasFocus)
        remote.press(.down)
        for _ in 0..<16 {
            let focused = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
            if app.staticTexts["Nature"].isHittable && focused.exists &&
               focused.frame.minY > app.staticTexts["Nature"].frame.maxY { break }
            remote.press(.down)
            downMovements += 1
        }
        XCTAssertTrue(app.staticTexts["Nature"].isHittable, "The last genre shelf must remain reachable")
        let focus = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
        XCTAssertTrue(focus.exists)
        XCTAssertTrue(focus.isHittable)
        let originalLabel = focus.label
        record("deep home genre", app)
        remote.press(.right, forDuration: 1)
        XCTAssertNotEqual(app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch.label,
                          originalLabel, "Right must move focus before reversing direction")
        remote.press(.left, forDuration: 1)
        let selected = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
        XCTAssertTrue(selected.isHittable)
        let selectedLabel = selected.label
        XCTAssertFalse(selectedLabel.isEmpty)
        record("deep home reversal", app)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10))
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Nature"].waitForExistence(timeout: 10))
        XCTAssertEqual(app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch.label,
                       selectedLabel, "Back must restore the selected Home card")
        record("deep home after detail", app)
        var upMovements = 0
        for _ in 0..<16 {
            if app.buttons["Search"].hasFocus { break }
            remote.press(.up)
            upMovements += 1
        }
        XCTAssertTrue(app.buttons["Search"].hasFocus)
        record("deep home return to search", app)
        print("REMOTE movements Search-to-Nature: \(downMovements) Down; Nature-to-Search: \(upMovements) Up")
    }
}
