import XCTest

final class RemoteDetailMovementTests: RemoteTestCase {
    @MainActor
    func testAlbumAndBackRestoreMusicFocus() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
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
        let focusedPlayerControl = app.buttons.matching(NSPredicate(
            format: "hasFocus == true AND (label == 'Back 15 seconds' OR label == 'Pause' OR label == 'Forward 30 seconds')"
        )).firstMatch
        XCTAssertTrue(focusedPlayerControl.waitForExistence(timeout: 10), "Audio player should receive focus")
        record("audio player", app)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Back 15 seconds"].waitForNonExistence(timeout: 10),
                      "Back should close the audio player")
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 10), "Back should return to the album")
        record("back from audio player", app)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Albums"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons[albumLabel].hasFocus)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Music"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Music"].hasFocus)
        record("back to Music tile", app)
    }

    @MainActor
    func testSettingsPlaybackBackRestoresRowAndTopBarFocus() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
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
        app.launch()
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
