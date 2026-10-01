import XCTest

final class RemotePlaybackCrashTests: RemoteTestCase {
    @MainActor func testHeldRemoteInputAndRepeatedPlaybackRemainUsable() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        remote.press(.home)
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 20), app.debugDescription)
        XCTAssertTrue(app.buttons["Movies"].waitForExistence(timeout: 30), app.debugDescription)
        for _ in 0..<30 where !app.buttons["Movies"].hasFocus { remote.press(.down) }
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 15))
        let card = app.buttons.matching(NSPredicate(format:
            "value == 'Unwatched' OR value == 'Watched' OR value BEGINSWITH 'Unwatched, continue'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 15))
        if !card.hasFocus { remote.press(.down) }
        remote.press(.select)
        let bookmarks = app.buttons["Bookmarks"]
        XCTAssertTrue(bookmarks.waitForExistence(timeout: 15), app.debugDescription)

        for attempt in 1...3 {
            let play = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'detail.play.'")).firstMatch
            XCTAssertTrue(play.waitForExistence(timeout: 10))
            for _ in 0..<12 where !play.hasFocus { remote.press(.up) }
            XCTAssertTrue(play.hasFocus, app.debugDescription)
            remote.press(.select)
            XCTAssertTrue(bookmarks.waitForNonExistence(timeout: 20))
            XCTAssertTrue(app.cells["Playback options"].waitForExistence(timeout: 30), app.debugDescription)
            let options = app.cells["Playback options"]
            remote.press(.down)
            remote.press(.up)
            for _ in 0..<12 where !options.hasFocus { remote.press(.right) }
            XCTAssertTrue(options.hasFocus, app.debugDescription)
            remote.press(.select)
            let preferences = app.cells["This title’s preferences"]
            XCTAssertTrue(preferences.waitForExistence(timeout: 10), app.debugDescription)
            remote.press(.menu)
            XCTAssertTrue(preferences.waitForNonExistence(timeout: 10))
            XCTAssertTrue(options.waitForExistence(timeout: 10))
            remote.press(.playPause)
            remote.press(.right, forDuration: 1)
            remote.press(.left, forDuration: 1)
            remote.press(.select)
            remote.press(.playPause)
            XCTAssertEqual(app.state, .runningForeground, "Remote seeking must not terminate playback")
            record("held remote playback \(attempt)", app)
            remote.press(.menu)
            XCTAssertTrue(bookmarks.waitForExistence(timeout: 15), "Back must return to the movie detail")
            XCTAssertEqual(app.state, .runningForeground)
        }
    }
}
