import XCTest

// Real tvOS loading and retry focus cannot be established by player unit tests.
// Each journey requires its own fresh loopback identity to avoid cached playback.
final class RemoteAudioStateTests: RemoteTestCase {
    private var fixtureEnabled = false
    override func setUpWithError() throws {
        try super.setUpWithError()
        continueAfterFailure = false
        let environment = ProcessInfo.processInfo.environment
        try XCTSkipUnless(environment["KINOSAIL_TV_FIXTURE_QA"] == "1"
                          && environment["KINOSAIL_TV_AUDIO_STATE_CASE"] == "1")
        fixtureEnabled = true
    }

    override func tearDown() async throws {
        if fixtureEnabled { try await state("loaded") }
        try await super.tearDown()
    }

    @MainActor func testPendingAudioRemovesSkeletonAndFocusesTransport() async throws {
        try await state("loaded", delay: 15)
        let app = openTrack()
        let loading = app.descendants(matching: .any).matching(identifier: "Opening audio…").firstMatch
        XCTAssertTrue(loading.waitForExistence(timeout: 5))
        record("audio pending", app)
        XCTAssertFalse(app.buttons["More"].exists)
        XCTAssertFalse(app.buttons["Playback options"].exists)
        try await state("loaded")
        XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 25))
        XCTAssertTrue(app.buttons["Pause"].hasFocus)
        XCUIRemote.shared.press(.select)
        XCTAssertTrue(app.buttons["Play"].waitForExistence(timeout: 5))
        XCTAssertFalse(loading.exists)
        record("audio loaded after pending", app)
    }

    @MainActor func testFailedAudioExposesRetryAndRecoversTransport() async throws {
        try await state("failed")
        let app = openTrack()
        let retry = app.buttons["Try again"]
        XCTAssertTrue(retry.waitForExistence(timeout: 15))
        XCTAssertTrue(retry.hasFocus)
        XCTAssertFalse(app.descendants(matching: .any).matching(identifier: "Opening audio…").firstMatch.exists)
        record("audio failed with reachable retry", app)
        try await state("loaded")
        XCUIRemote.shared.press(.select)
        XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 20))
        XCTAssertTrue(app.buttons["Pause"].hasFocus)
        XCUIRemote.shared.press(.select)
        XCTAssertTrue(app.buttons["Play"].waitForExistence(timeout: 5))
        XCTAssertFalse(retry.exists)
        record("audio recovered after retry", app)
        XCUIRemote.shared.press(.menu)
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 10))
    }

    @MainActor private func openTrack() -> XCUIApplication {
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
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 10))
        remote.press(.select)
        return app
    }

    private func state(_ mode: String, delay: Int = 0) async throws {
        let identity = try XCTUnwrap(ProcessInfo.processInfo.environment["KINOSAIL_TV_QUALITY_FIXTURE_ID"])
        XCTAssertNotNil(identity.range(of: "^tv-polish-qa-[a-z0-9-]{1,64}$", options: .regularExpression))
        let url = try XCTUnwrap(URL(string: "http://127.0.0.1:38359/qa/state?mode=\(mode)&delay=\(delay)&target=playback&identity=\(identity)"))
        let (_, response) = try await URLSession.shared.data(from: url)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
    }
}
