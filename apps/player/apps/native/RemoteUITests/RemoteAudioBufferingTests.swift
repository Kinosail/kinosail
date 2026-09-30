import XCTest

// The loopback fixture withholds media bytes, while allowing metadata and artwork.
// Labels must match the next action in both Now playing and the mini player.
final class RemoteAudioBufferingTests: RemoteTestCase {
    override func tearDown() async throws {
        _ = try await URLSession.shared.data(from: URL(string: "http://127.0.0.1:4279/qa/state?mode=loaded&delay=0&target=library&profile=qa")!)
        try await super.tearDown()
    }

    @MainActor func testWaitingAudioLabelsMatchPauseAndResumeActions() async throws {
        let (_, response) = try await URLSession.shared.data(from: URL(string: "http://127.0.0.1:4279/qa/state?mode=loaded&delay=35&target=media&profile=qa")!)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        remote.press(.right)
        remote.press(.right)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Albums"].waitForExistence(timeout: 10))
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 10))
        remote.press(.select)
        XCTAssertTrue(app.buttons["Back 15 seconds"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Play"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 3))
        XCTAssertTrue(app.staticTexts["Opening audio…"].exists)
        record("audio buffering with Pause action", app)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Play"].waitForExistence(timeout: 3))
        remote.press(.select)
        XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 3))
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 5))
        XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 3))
        record("mini player buffering with Pause action", app)
        remote.press(.down)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Pause"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Play"].waitForExistence(timeout: 3))
    }
}
