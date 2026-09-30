import XCTest

final class RemotePlaybackOptionsTests: RemoteTestCase {
    @MainActor func testPlaybackOptionsFitAndReturnToVideo() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15), app.debugDescription)
        remote.press(.down)
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        let card = app.buttons.matching(NSPredicate(format: "label != '' AND NOT label ENDSWITH 'titles'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 10))
        if !card.hasFocus { remote.press(.down) }
        remote.press(.select)
        XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10))
        remote.press(.select)
        let options = app.cells["Playback options"]
        XCTAssertTrue(options.waitForExistence(timeout: 20), app.debugDescription)
        remote.press(.down)
        remote.press(.up)
        for _ in 0..<12 where !options.hasFocus { remote.press(.right) }
        XCTAssertTrue(options.hasFocus, app.debugDescription)
        remote.press(.select)
        let preferences = app.cells["This title’s preferences"]
        let done = app.buttons["Done"]
        XCTAssertTrue(preferences.waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(done.exists)
        record("playback options", app)
        XCTAssertLessThan(done.frame.maxX, app.frame.width * 0.25,
                          "Options need a full-screen presentation to fit the TV settings layout")
        XCTAssertTrue(app.frame.contains(preferences.frame), "The title preferences control must fit on screen")
        XCTAssertFalse(app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Couldn’t save your position'")).firstMatch.exists)
        for _ in 0..<12 where !preferences.hasFocus { remote.press(.down) }
        XCTAssertTrue(preferences.hasFocus, app.debugDescription)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Speed"].waitForExistence(timeout: 10), app.debugDescription)
        record("title playback preferences", app)
        remote.press(.menu)
        XCTAssertTrue(preferences.waitForExistence(timeout: 10))
        remote.press(.menu)
        XCTAssertTrue(preferences.waitForNonExistence(timeout: 10), "Back should dismiss options")
        XCTAssertTrue(options.waitForExistence(timeout: 10), "Playback must stay available after dismissing options")
        record("video after options", app)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10), "Back should still return to the movie")
    }
}
