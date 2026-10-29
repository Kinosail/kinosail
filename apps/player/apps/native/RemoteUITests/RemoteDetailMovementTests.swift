import XCTest

final class RemoteDetailMovementTests: RemoteTestCase {
    @MainActor
    func testAlbumAndBackRestoreMusicFocus() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        remote.press(.right)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Music"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Albums"].waitForExistence(timeout: 10))
        let album = app.buttons.matching(NSPredicate(format: "label != '' AND label != 'All music tracks'")).firstMatch
        XCTAssertTrue(album.waitForExistence(timeout: 10))
        XCTAssertTrue(album.hasFocus, "The first album should have initial focus")
        let albumLabel = album.label
        XCTAssertFalse(albumLabel.isEmpty)
        XCTAssertNotEqual(albumLabel, "All music tracks")
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
            .waitForExistence(timeout: 5), "The first track should have initial focus")
        record("album tracks", app)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Back 15 seconds"].waitForExistence(timeout: 20), "Audio player should open")
        XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Pause"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Play"].waitForExistence(timeout: 5))
        let focusedPlayerControl = app.buttons.matching(NSPredicate(
            format: "hasFocus == true AND (label == 'Back 15 seconds' OR label == 'Play' OR label == 'Forward 30 seconds')"
        )).firstMatch
        XCTAssertTrue(focusedPlayerControl.waitForExistence(timeout: 10), "Audio player should receive focus")
        record("audio player", app)
        XCTAssertEqual(app.buttons["Previous"].frame.midX, app.buttons["Back 15 seconds"].frame.midX, accuracy: 8)
        XCTAssertEqual(app.buttons["Next"].frame.midX, app.buttons["Forward 30 seconds"].frame.midX, accuracy: 8)
        XCTAssertTrue(app.buttons["Play"].hasFocus)
        let path: [(XCUIRemote.Button, String)] = [
            (.left, "Back 15 seconds"), (.right, "Play"), (.right, "Forward 30 seconds"),
            (.down, "Next"), (.left, "Previous"), (.down, "Shuffle off"), (.right, "Repeat: off"),
            (.right, "Sleep timer"), (.left, "Repeat: off"), (.left, "Shuffle off"),
            (.up, "Previous")
        ]
        for (arrow, expected) in path {
            remote.press(arrow)
            record("audio arrow to \(expected)", app)
            XCTAssertTrue(focusedLabels(app).contains(expected),
                          "Expected \(expected); actual: \(focusedLabels(app))")
        }
        verifyAudioMenus(app, remote)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Next"].hasFocus)
        remote.press(.up)
        XCTAssertTrue(app.buttons["Forward 30 seconds"].hasFocus)
        remote.press(.up)
        record("audio Up to Options", app)
        let options = app.buttons["Options"]
        XCTAssertTrue(options.hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.cells["This title’s preferences"].waitForExistence(timeout: 10))
        record("audio Playback options", app)
        XCTAssertTrue(app.frame.contains(app.cells["This title’s preferences"].frame))
        XCTAssertFalse(app.buttons["Done"].exists, "The options panel closes with Menu, not a Done button")
        XCTAssertGreaterThan(app.cells["This title’s preferences"].frame.minX, app.frame.width * 0.5,
                             "Audio options need the same right-side panel as video")
        remote.press(.menu)
        XCTAssertTrue(options.waitForExistence(timeout: 10))
        XCTAssertTrue(options.hasFocus)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Back 15 seconds"].waitForNonExistence(timeout: 10),
                      "Back should close the audio player")
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 10), "Back should return to the album")
        record("back from audio player", app)
        remote.press(.down)
        let mini = app.buttons["Now playing: Evening Light"]
        XCTAssertTrue(mini.hasFocus)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Play"].hasFocus)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Stop"].hasFocus)
        remote.press(.left)
        remote.press(.left)
        XCTAssertTrue(mini.hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Back 15 seconds"].waitForExistence(timeout: 10))
        remote.press(.menu)
        XCTAssertTrue(mini.waitForExistence(timeout: 10))
        XCTAssertTrue(mini.hasFocus)
        record("mini player return focus", app)
        remote.press(.right)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Stop"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(mini.waitForNonExistence(timeout: 5))
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Albums"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons[albumLabel].hasFocus)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Music"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Music"].hasFocus)
        record("back to Music tile", app)
    }

    @MainActor private func verifyAudioMenus(_ app: XCUIApplication, _ remote: XCUIRemote) {
        remote.press(.down)
        XCTAssertTrue(app.buttons["Shuffle off"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Shuffle on"].waitForExistence(timeout: 5))
        remote.press(.select)
        XCTAssertTrue(app.buttons["Shuffle off"].waitForExistence(timeout: 5))
        remote.press(.right)
        for (mode, downs) in [("all", 1), ("one", 2), ("off", 0)] {
            remote.press(.select)
            XCTAssertTrue(app.descendants(matching: .any)["All"].waitForExistence(timeout: 5))
            for _ in 0..<downs { remote.press(.down) }
            record("audio Repeat menu to \(mode)", app)
            remote.press(.select)
            let settled = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                self.focusedLabels(app).contains("Repeat: \(mode)")
            }, object: nil)
            XCTAssertEqual(XCTWaiter.wait(for: [settled], timeout: 5), .completed)
        }
        remote.press(.right)
        let deadline = app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Pauses at '")).firstMatch
        for (index, choice) in ["15 minutes", "30 minutes", "45 minutes", "60 minutes", "90 minutes", "Turn off"].enumerated() {
            remote.press(.select)
            XCTAssertTrue(app.descendants(matching: .any)["15 minutes"].waitForExistence(timeout: 5))
            for _ in 0..<index { remote.press(.down) }
            record("audio Sleep timer menu to \(choice)", app)
            let beforeSelection = Date()
            remote.press(.select)
            if choice == "Turn off" { XCTAssertTrue(deadline.waitForNonExistence(timeout: 5)) }
            else {
                let minutes = [15, 30, 45, 60, 90][index]
                let expected = [beforeSelection, Date()].map {
                    "Pauses at \($0.addingTimeInterval(Double(minutes) * 60).formatted(date: .omitted, time: .shortened))"
                }
                let applied = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                    deadline.exists && expected.contains(deadline.label)
                }, object: nil)
                XCTAssertEqual(XCTWaiter.wait(for: [applied], timeout: 5), .completed)
            }
            let settled = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                self.focusedLabels(app).contains("Sleep timer")
            }, object: nil)
            XCTAssertEqual(XCTWaiter.wait(for: [settled], timeout: 5), .completed)
        }
        remote.press(.left)
        remote.press(.left)
        remote.press(.up)
        XCTAssertTrue(app.buttons["Previous"].hasFocus)
    }

    @MainActor
    func testSettingsPlaybackBackRestoresRowAndTopBarFocus() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        let settings = app.buttons["Settings"]
        XCTAssertTrue(settings.waitForExistence(timeout: 15), app.debugDescription)
        remote.press(.right)
        XCTAssertTrue(settings.hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Playback"].waitForExistence(timeout: 10), app.debugDescription)

        var focusPath: [String] = []
        for _ in 0..<12 {
            let label = focusedLabels(app).joined(separator: ", ")
            focusPath.append(label)
            if label.contains("Playback") { break }
            remote.press(.down)
        }
        print("REMOTE Settings focus path: \(focusPath)")
        XCTAssertTrue(focusPath.last?.contains("Playback") == true, "Playback must be reachable with Down")
        record("settings playback row", app)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Playback preferences"].waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(app.staticTexts["Speed"].waitForExistence(timeout: 10), app.debugDescription)
        record("playback preferences", app)
        XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains("Speed") }))
        for label in ["Audio language", "Subtitles", "Boost Dialog", "Normalize Loudness"] {
            remote.press(.down)
            record("preferences Down to \(label)", app)
            XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains(label) }))
        }
        XCTAssertFalse(app.buttons["Save preferences"].exists, "Changes save automatically")
        remote.press(.down)
        XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains("Normalize Loudness") }), "Normalize Loudness is the last row")
        for label in ["Boost Dialog", "Subtitles", "Audio language", "Speed"] {
            remote.press(.up)
            XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains(label) }))
        }
        record("preferences all rows and return to Speed", app)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Playback"].waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains("Playback") }))
        record("back to settings playback", app)
        remote.press(.menu)
        XCTAssertTrue(settings.waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(settings.hasFocus)
        record("back to settings top bar", app)
    }

    @MainActor
    func testHomeDirectionalRowsAndLeftEdge() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        let search = app.buttons["Search"]
        XCTAssertTrue(search.waitForExistence(timeout: 15), app.debugDescription)
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 10), app.debugDescription)
        remote.press(.down)
        let firstCard = focusedLabels(app).first ?? ""
        XCTAssertFalse(firstCard.isEmpty)
        XCTAssertNotEqual(firstCard, "Movies")
        record("first watching card", app)
        remote.press(.up)
        XCTAssertTrue(search.hasFocus)
        remote.press(.down)
        XCTAssertEqual(focusedLabels(app).first, firstCard)
        remote.press(.right)
        XCTAssertNotEqual(focusedLabels(app).first, firstCard)
        record("next watching card", app)
        remote.press(.left)
        XCTAssertEqual(focusedLabels(app).first, firstCard)
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.left)
        XCTAssertTrue(app.buttons["Movies"].hasFocus, "Left should stop at the first Browse tile")
        remote.press(.down)
        let recent = focusedLabels(app).first ?? ""
        XCTAssertFalse(recent.isEmpty)
        XCTAssertNotEqual(recent, "Movies")
        record("recent movies", app)
        remote.press(.up)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.up)
        XCTAssertEqual(focusedLabels(app).first, firstCard)
        remote.press(.up)
        XCTAssertTrue(search.hasFocus)
        record("back to Search with Up", app)
    }

}
