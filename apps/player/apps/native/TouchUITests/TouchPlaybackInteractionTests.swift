import XCTest

// Run against the disposable loopback fixture in scripts/ios-playback-fixture.py.
final class TouchPlaybackInteractionTests: XCTestCase {
    @MainActor func testOpeningVideoHasNoPlayAndKeepsCloseAfterBackgroundTap() async throws {
        let app = try await openVideo(mode: "pending")
        XCTAssertFalse(app.buttons["Play"].exists, app.debugDescription)
        record("opening video", app)
        background(app, x: 0.75, y: 0.3).tap()
        try await assertCloseHittable(app)
        app.buttons["Close player"].tap()
        XCTAssertTrue(app.buttons["Details"].waitForExistence(timeout: 10))
    }

    @MainActor func testWaitingForFirstFrameHasNoPlayOrPause() async throws {
        let app = try await openVideo(mode: "preparing")
        // Metadata is available, but the fixture withholds all video bytes.
        XCTAssertTrue(app.staticTexts["−1:00"].waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertFalse(app.buttons["Play"].exists, app.debugDescription)
        XCTAssertFalse(app.buttons["Pause"].exists, app.debugDescription)
        try await assertCloseHittable(app)
        record("waiting for first frame", app)
        app.buttons["Close player"].tap()
        XCTAssertTrue(app.buttons["Details"].waitForExistence(timeout: 10))
    }

    @MainActor func testSwipeDownAndEdgeBackReturnWhileOpening() async throws {
        for direction in ["down", "back"] {
            let app = try await openVideo(mode: "pending")
            let start = background(app, x: direction == "back" ? 0.02 : 0.75, y: 0.3)
            let end = background(app, x: direction == "back" ? 0.65 : 0.75,
                                 y: direction == "back" ? 0.3 : 0.65)
            start.press(forDuration: 0.05, thenDragTo: end)
            XCTAssertTrue(app.buttons["Details"].waitForExistence(timeout: 10), direction)
            XCTAssertFalse(app.buttons["Close player"].exists)
        }
    }

    @MainActor func testLandscapePhysicalEdgeBackReturnsWhileOpening() async throws {
        let app = try await openVideo(mode: "pending")
        XCUIDevice.shared.orientation = .landscapeLeft
        defer { XCUIDevice.shared.orientation = .portrait }
        for _ in 0..<50 {
            let frame = app.windows.firstMatch.frame
            if frame.width > frame.height { break }
            try await Task.sleep(for: .milliseconds(100))
        }
        XCTAssertGreaterThan(app.windows.firstMatch.frame.width, app.windows.firstMatch.frame.height)
        XCTAssertTrue(app.staticTexts["Opening video…"].waitForExistence(timeout: 5))
        record("landscape opening video", app)
        background(app, x: 0.01, y: 0.3).press(forDuration: 0.05,
                                             thenDragTo: background(app, x: 0.65, y: 0.3))
        XCTAssertTrue(app.buttons["Details"].waitForExistence(timeout: 10))
    }

    @MainActor func testFailedVideoHasRetryAndCloseWithoutLoadingOrPlay() async throws {
        let app = try await openVideo(mode: "failed")
        XCTAssertTrue(app.buttons["Try again"].waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertFalse(app.staticTexts["Opening video…"].exists)
        XCTAssertFalse(app.buttons["Play"].exists)
        try await assertCloseHittable(app)
        record("failed video", app)
        app.buttons["Close player"].tap()
        XCTAssertTrue(app.buttons["Details"].waitForExistence(timeout: 10))
    }

    @MainActor func testLoadedVideoHasWorkingPauseAndPlay() async throws {
        let app = try await openVideo(mode: "loaded")
        let pause = app.buttons["Pause"]
        XCTAssertTrue(pause.waitForExistence(timeout: 15), app.debugDescription)
        XCTAssertTrue(pause.isEnabled)
        XCTAssertFalse(app.staticTexts["Opening video…"].exists)
        record("loaded video", app)
        pause.tap()
        XCTAssertTrue(app.buttons["Play"].waitForExistence(timeout: 5))
        app.buttons["Play"].tap()
        XCTAssertTrue(pause.waitForExistence(timeout: 5))
        app.buttons["Close player"].tap()
        XCTAssertTrue(app.buttons["Details"].waitForExistence(timeout: 10))
    }

    @MainActor private func openVideo(mode: String) async throws -> XCUIApplication {
        continueAfterFailure = false
        XCUIDevice.shared.orientation = .portrait
        let (_, response) = try await URLSession.shared.data(from:
            URL(string: "http://127.0.0.1:4281/qa/state?mode=\(mode)")!)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        let address = app.textFields["Server address"]
        if address.waitForExistence(timeout: 3) {
            address.tap()
            address.typeText("http://127.0.0.1:4281")
            app.buttons["Connect"].tap()
        }
        let play = app.buttons["Play"].firstMatch
        XCTAssertTrue(play.waitForExistence(timeout: 20), app.debugDescription)
        play.tap()
        XCTAssertTrue(app.buttons["Close player"].waitForExistence(timeout: 10), app.debugDescription)
        return app
    }

    @MainActor private func background(_ app: XCUIApplication, x: CGFloat, y: CGFloat) -> XCUICoordinate {
        app.windows.firstMatch.coordinate(withNormalizedOffset: CGVector(dx: x, dy: y))
    }

    @MainActor private func assertCloseHittable(_ app: XCUIApplication) async throws {
        let close = app.buttons["Close player"]
        for _ in 0..<50 {
            if close.isHittable { break }
            try await Task.sleep(for: .milliseconds(100))
        }
        XCTAssertTrue(close.isHittable, app.debugDescription)
    }

    @MainActor private func record(_ name: String, _ app: XCUIApplication) {
        let attachment = XCTAttachment(screenshot: app.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }
}
