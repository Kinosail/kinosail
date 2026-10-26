import XCTest

final class RemotePlaybackOptionsTests: RemoteTestCase {
    @MainActor func testPlaybackOptionsFitAndReturnToVideo() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15), app.debugDescription)
        remote.press(.down)
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS 'After the Rain'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 10))
        if !card.hasFocus { remote.press(.down) }
        remote.press(.select)
        XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10))
        remote.press(.select)
        let options = app.cells["Playback options"]
        XCTAssertTrue(options.waitForExistence(timeout: 20), app.debugDescription)
        remote.press(.select)
        remote.press(.up)
        for _ in 0..<12 where !options.hasFocus { remote.press(.right) }
        XCTAssertTrue(options.hasFocus, app.debugDescription)
        XCTAssertFalse(app.cells["Seek with preview"].exists, "Scrubbing previews belong to the normal scrub bar, not a separate button")
        remote.press(.select)
        let preferences = app.cells["This title’s preferences"]
        XCTAssertTrue(preferences.waitForExistence(timeout: 10), app.debugDescription)
        record("playback options", app)
        XCTAssertTrue(app.staticTexts["After the Rain"].exists, "The panel should name the playing title")
        XCTAssertFalse(app.buttons["Done"].exists, "Back closes the options panel, so it needs no Done button")
        XCTAssertGreaterThan(preferences.frame.minX, app.frame.width * 0.5,
                             "Options should sit in a side panel so the paused video stays visible")
        XCTAssertTrue(app.frame.contains(preferences.frame), "The title preferences control must fit on screen")
        XCTAssertFalse(app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Couldn’t save your position'")).firstMatch.exists)
        XCTAssertTrue(app.staticTexts["Playing the original file"].exists, "The panel should say which version is playing")
        XCTAssertFalse(app.cells.matching(NSPredicate(format: "label BEGINSWITH 'Playing '")).firstMatch.exists,
                       "Playback status is information, not a focusable row")
        let speed = app.cells.matching(NSPredicate(format: "label BEGINSWITH 'Playback speed'")).firstMatch
        XCTAssertTrue(speed.exists, app.debugDescription)
        XCTAssertTrue(speed.label.contains("Normal"), "Each option row shows its current value, got \(speed.label)")
        for _ in 0..<12 where !speed.hasFocus { remote.press(.down) }
        XCTAssertTrue(speed.hasFocus, app.debugDescription)
        let status = app.staticTexts["Playing the original file"]
        XCTAssertGreaterThanOrEqual(speed.frame.minX, status.frame.minX - 32, "A focused row must line up with the panel content, not run past its edge")
        record("playback options speed focus", app)
        remote.press(.select)
        let normal = app.cells.matching(NSPredicate(format: "label BEGINSWITH 'Normal'")).firstMatch
        XCTAssertTrue(normal.waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(normal.hasFocus, "A picker opens on the current choice")
        XCTAssertTrue(normal.label.contains("Selected"), "The current choice must read as selected, got \(normal.label)")
        record("playback speed picker", app)
        remote.press(.menu)
        XCTAssertTrue(speed.waitForExistence(timeout: 10))
        for _ in 0..<12 where !preferences.hasFocus { remote.press(.down) }
        XCTAssertTrue(preferences.hasFocus, app.debugDescription)
        remote.press(.select)
        let titleSpeed = app.cells.matching(NSPredicate(format: "label BEGINSWITH 'Speed'")).firstMatch
        XCTAssertTrue(titleSpeed.waitForExistence(timeout: 10), app.debugDescription)
        record("title playback preferences", app)
        XCTAssertTrue(titleSpeed.label.contains("1×"), "The Speed row shows its value on the same row, got \(titleSpeed.label)")
        for label in ["Audio language", "Subtitles"] {
            let row = app.cells.matching(NSPredicate(format: "label BEGINSWITH %@", label)).firstMatch
            XCTAssertTrue(row.exists && row.label.contains("Automatic"), "The \(label) row shows its value, got \(row.label)")
        }
        XCTAssertFalse(app.buttons["Save preferences"].exists, "Changes save automatically")
        XCTAssertFalse(app.buttons["Use profile defaults"].exists, "There is nothing to reset while the title uses the profile defaults")
        XCTAssertFalse(app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'Downloads'")).firstMatch.exists,
                       "Apple TV has no downloads")
        XCTAssertFalse(app.cells.matching(NSPredicate(format: "label BEGINSWITH 'This title'")).firstMatch.exists,
                       "Status text is information, not a focusable row")
        XCTAssertTrue(app.frame.contains(titleSpeed.frame))
        for _ in 0..<6 where !titleSpeed.hasFocus { remote.press(.up) }
        XCTAssertTrue(titleSpeed.hasFocus, app.debugDescription)
        remote.press(.select)
        let faster = app.cells.matching(NSPredicate(format: "label BEGINSWITH '1.5×'")).firstMatch
        XCTAssertTrue(faster.waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(app.cells.matching(NSPredicate(format: "label BEGINSWITH '1×'")).firstMatch.label.contains("Selected"))
        for _ in 0..<12 where !faster.hasFocus { remote.press(.down) }
        XCTAssertTrue(faster.hasFocus, app.debugDescription)
        remote.press(.select)
        XCTAssertTrue(titleSpeed.waitForExistence(timeout: 10), app.debugDescription)
        let saved = NSPredicate(format: "label CONTAINS '1.5×'")
        XCTAssertEqual(XCTWaiter.wait(for: [XCTNSPredicateExpectation(predicate: saved, object: titleSpeed)], timeout: 10), .completed,
                       "The choice saves without a Save button, got \(titleSpeed.label)")
        let reset = app.buttons["Use profile defaults"]
        XCTAssertTrue(reset.waitForExistence(timeout: 10), "A title with its own preferences offers a reset")
        record("title preferences saved", app)
        for _ in 0..<12 where !focusedLabels(app).contains("Use profile defaults") { remote.press(.down) }
        XCTAssertTrue(focusedLabels(app).contains("Use profile defaults"), app.debugDescription)
        remote.press(.select)
        XCTAssertTrue(reset.waitForNonExistence(timeout: 10), "After the reset the title uses the profile defaults again")
        XCTAssertTrue(titleSpeed.label.contains("1×") && !titleSpeed.label.contains("1.5×"), titleSpeed.label)
        record("title preferences reset", app)
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
