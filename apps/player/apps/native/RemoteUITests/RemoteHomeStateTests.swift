import XCTest

final class RemoteHomeStateTests: RemoteTestCase {
    private var fixtureEnabled = false
    override func setUpWithError() throws {
        try super.setUpWithError()
        try XCTSkipUnless(ProcessInfo.processInfo.environment["KINOSAIL_TV_FIXTURE_QA"] == "1",
                          "Requires the disposable loopback catalog and QA Viewer Profile.")
    }
    override func tearDown() async throws {
        if fixtureEnabled { try await state("loaded") }
        try await super.tearDown()
    }

    @MainActor func testPendingHomeKeepsNavigationReachableAndRemovesSkeleton() async throws {
        continueAfterFailure = false
        try requireMode("pending")
        try await state("loaded", delay: 15)
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 5))
        XCTAssertTrue(app.descendants(matching: .any)["Loading your library…"].waitForExistence(timeout: 5))
        record("quality-home-pending", app)
        let pending = app.screenshot().image
        let searchFrame = app.buttons["Search"].frame
        XCUIRemote.shared.press(.right)
        XCTAssertTrue(app.buttons["Settings"].hasFocus)
        XCUIRemote.shared.press(.left)
        XCTAssertTrue(app.buttons["Search"].hasFocus)
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 30))
        XCTAssertFalse(app.descendants(matching: .any)["Loading your library…"].exists)
        XCTAssertTrue(app.buttons["Search"].hasFocus, "Loaded titles must not steal focus from a viewer already using the top bar")
        XCTAssertEqual(app.buttons["Search"].frame, searchFrame)
        record("quality-home-pending-loaded", app)
        let heading = app.staticTexts["Continue watching"].frame
        let frames: [CGRect] = app.buttons.allElementsBoundByIndex.map(\.frame)
        let cards = frames.filter { (frame: CGRect) -> Bool in
            frame.width > 360 && frame.width < 420 && frame.minY > heading.maxY && frame.minY < heading.maxY + 160
        }.sorted { $0.minX < $1.minX }
        XCTAssertGreaterThanOrEqual(cards.count, 4)
        let fill = pendingColor(pending, app, cards[0].midX, cards[0].midY)
        for card in cards.prefix(4) {
            for x in [card.minX + 12, card.maxX - 12] {
                let color = pendingColor(pending, app, x, card.midY)
                let close = zip(color, fill).allSatisfy { (pair: (UInt8, UInt8)) -> Bool in abs(Int(pair.0) - Int(pair.1)) <= 4 }
                XCTAssertTrue(close,
                              "The skeleton should cover \(Int(x)),\(Int(card.midY)) where a loaded card appears: \(color) vs \(fill)")
            }
        }
    }

    @MainActor func testEmptyHomeBackReachesNavigation() async throws {
        continueAfterFailure = false
        try requireMode("empty")
        try await state("empty")
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        XCTAssertTrue(app.staticTexts["Your library is ready"].waitForExistence(timeout: 15))
        XCTAssertFalse(app.descendants(matching: .any)["Loading your library…"].exists)
        XCUIRemote.shared.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        record("quality-home-empty", app)
        for title in ["Shows", "Music", "Audiobooks", "Photos"] {
            XCUIRemote.shared.press(.right)
            XCTAssertTrue(app.buttons[title].hasFocus)
        }
        XCUIRemote.shared.press(.up)
        XCTAssertTrue(app.buttons["Settings"].hasFocus)
        XCUIRemote.shared.press(.down)
        XCTAssertTrue(app.buttons["Photos"].hasFocus)
        XCUIRemote.shared.press(.menu)
        XCTAssertTrue(app.buttons["Search"].hasFocus)
        record("quality-home-empty-back", app)
    }

    @MainActor func testFailedHomeBackAndRetryRemainUsable() async throws {
        continueAfterFailure = false
        try requireMode("failed")
        try await state("failed")
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        XCTAssertTrue(app.buttons["Try again"].waitForExistence(timeout: 15))
        XCTAssertFalse(app.descendants(matching: .any)["Loading your library…"].exists)
        XCUIRemote.shared.press(.right)
        XCTAssertTrue(app.buttons["Settings"].hasFocus)
        XCUIRemote.shared.press(.down)
        XCTAssertTrue(app.buttons["Try again"].hasFocus)
        record("quality-home-failed", app)
        XCUIRemote.shared.press(.menu)
        XCTAssertTrue(app.buttons["Search"].hasFocus)
        record("quality-home-failed-back", app)
        XCUIRemote.shared.press(.down)
        XCTAssertTrue(app.buttons["Try again"].hasFocus)
        try await state("loaded")
        XCUIRemote.shared.press(.select)
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        XCTAssertFalse(app.buttons["Try again"].exists)
        record("quality-home-failed-recovered", app)
    }

    private func requireMode(_ mode: String) throws {
        try XCTSkipUnless(ProcessInfo.processInfo.environment["KINOSAIL_TV_QUALITY_FIXTURE_MODE"] == mode,
                          "Requires a fresh disposable \(mode) fixture session.")
        fixtureEnabled = true
    }

    private func state(_ mode: String, delay: Int = 0) async throws {
        let identity = ProcessInfo.processInfo.environment["KINOSAIL_TV_QUALITY_FIXTURE_ID"] ?? "tv-polish-qa"
        let url = URL(string: "http://127.0.0.1:38359/qa/state?mode=\(mode)&delay=\(delay)&target=library&identity=\(identity)")!
        let (_, response) = try await URLSession.shared.data(from: url)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
    }

    @MainActor
    private func pendingColor(_ image: UIImage, _ app: XCUIApplication, _ x: CGFloat, _ y: CGFloat) -> [UInt8] {
        Array(samplePixel(image, x: x / app.frame.width, y: y / app.frame.height).prefix(3))
    }
}
