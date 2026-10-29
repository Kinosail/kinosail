import XCTest

final class EmptyLibraryTests: XCTestCase {
    @MainActor private func switchServer(_ app: XCUIApplication, to server: String, token: String) async throws {
        app.terminate()
        app.launch()
        if !app.textFields["Server address"].waitForExistence(timeout: 5) {
            let more = tourTab("More", in: app)
            XCTAssertTrue(more.waitForExistence(timeout: 60), app.debugDescription)
            tourReveal(more, in: app)
            more.tap()
            more.tap()
            let settings = app.buttons["Settings"].firstMatch
            XCTAssertTrue(settings.waitForExistence(timeout: 10), app.debugDescription)
            for _ in 0..<5 where !settings.isHittable { app.swipeUp() }
            settings.tap()
            let signOut = app.buttons["Sign out"].firstMatch
            for _ in 0..<8 where !(signOut.exists && signOut.isHittable) { app.swipeUp() }
            signOut.tap()
            let confirm = app.alerts.buttons["Sign out"]
            XCTAssertTrue(confirm.waitForExistence(timeout: 10), app.debugDescription)
            confirm.tap()
            XCTAssertTrue(app.textFields["Server address"].waitForExistence(timeout: 20), app.debugDescription)
        }
        try await tourConnect(app, server: server, token: token, records: false)
        XCTAssertTrue(tourTab("Home", in: app).waitForExistence(timeout: 90), app.debugDescription)
    }

    @MainActor private func assertSettled(_ name: String, _ app: XCUIApplication) {
        let loading = app.descendants(matching: .any).matching(NSPredicate(format: "label BEGINSWITH 'Loading'")).firstMatch
        let deadline = Date().addingTimeInterval(20)
        while loading.exists && Date() < deadline { sleep(1) }
        XCTAssertFalse(loading.exists, "\(name) keeps a loading placeholder in an empty library")
        let failure = app.staticTexts["Couldn't load this content"]
        XCTAssertFalse(failure.exists, "\(name) reports a failure for an empty library")
    }

    @MainActor func testEmptyLibraryShowsDesignedEmptyStates() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let empty = environment["KINOSAIL_EMPTY_SERVER"], !empty.isEmpty,
              let populated = environment["KINOSAIL_DESIGN_TOUR_SERVER"], !populated.isEmpty else {
            throw XCTSkip("Set KINOSAIL_EMPTY_SERVER to a test Server without media and KINOSAIL_DESIGN_TOUR_SERVER to restore.")
        }
        continueAfterFailure = true
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        try await switchServer(app, to: empty, token: environment["KINOSAIL_EMPTY_TOKEN"] ?? "")
        assertSettled("Home", app)
        XCTAssertTrue(app.staticTexts["Your library is ready"].waitForExistence(timeout: 10), app.debugDescription)
        tourRecord("e1 empty home", app)
        for (tab, title) in [("Shows", "Nothing here yet"), ("Movies", "Nothing here yet"), ("Search", "Nothing here yet")] {
            let button = tourTab(tab, in: app)
            tourReveal(button, in: app)
            button.tap()
            assertSettled(tab, app)
            XCTAssertTrue(app.staticTexts[title].waitForExistence(timeout: 15), "\(tab): \(app.debugDescription)")
            tourRecord("e2 empty \(tab.lowercased())", app)
        }
        tourTab("More", in: app).tap()
        let list = app.buttons["My List"].firstMatch
        if list.waitForExistence(timeout: 5) {
            list.tap()
            assertSettled("My List", app)
            XCTAssertTrue(app.staticTexts["My List is empty"].waitForExistence(timeout: 15), app.debugDescription)
            tourRecord("e3 empty my list", app)
            tourBack(app)
        }
        try await switchServer(app, to: populated, token: environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? "")
    }
}
