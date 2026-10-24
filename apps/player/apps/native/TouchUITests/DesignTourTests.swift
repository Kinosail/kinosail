import XCTest

final class DesignTourTests: XCTestCase {
    @MainActor func testCaptureDesignTour() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_DESIGN_TOUR_SERVER"], !server.isEmpty else {
            throw XCTSkip("Set KINOSAIL_DESIGN_TOUR_SERVER to a populated test Server.")
        }
        continueAfterFailure = true
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        try await tourConnect(app, server: server, token: environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? "", records: true)
        try await tour(app)
    }

    @MainActor func testCaptureDownloadJourney() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_DESIGN_TOUR_SERVER"], !server.isEmpty else {
            throw XCTSkip("Set KINOSAIL_DESIGN_TOUR_SERVER to a populated test Server.")
        }
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        try await tourConnect(app, server: server, token: environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? "", records: false)
        let home = tourTab("Home", in: app)
        XCTAssertTrue(home.waitForExistence(timeout: 90), app.debugDescription)
        let details = app.buttons["Details"].firstMatch
        XCTAssertTrue(details.waitForExistence(timeout: 20), app.debugDescription)
        details.tap()
        if app.buttons["Downloaded"].waitForExistence(timeout: 5) {
            app.buttons["Downloaded"].tap()
            removeDownload(in: app)
            app.navigationBars.buttons.firstMatch.tap()
        }
        let start = app.buttons["Download"].firstMatch
        XCTAssertTrue(start.waitForExistence(timeout: 15), app.debugDescription)
        start.tap()
        let add = app.buttons["Add to Downloads"]
        XCTAssertTrue(add.waitForExistence(timeout: 10), app.debugDescription)
        for _ in 0..<20 where !add.isEnabled { sleep(1) }
        tourRecord("d1 download options", app)
        add.tap()
        let status = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Download ' OR label == 'Downloaded'")).firstMatch
        XCTAssertTrue(status.waitForExistence(timeout: 20), app.debugDescription)
        tourRecord("d2 detail downloading", app)
        status.tap()
        XCTAssertTrue(app.navigationBars["Downloads"].waitForExistence(timeout: 10), app.debugDescription)
        sleep(1)
        tourRecord("d3 downloads in progress", app)
        let ready = app.staticTexts["Ready to watch"]
        XCTAssertTrue(ready.waitForExistence(timeout: 240), app.debugDescription)
        sleep(1)
        tourRecord("d4 downloads ready", app)
        app.navigationBars.buttons.firstMatch.tap()
        XCTAssertTrue(app.buttons["Downloaded"].waitForExistence(timeout: 10), app.debugDescription)
        tourRecord("d5 detail downloaded", app)
        app.navigationBars.buttons.firstMatch.tap()
        sleep(2)
        app.swipeUp()
        sleep(1)
        tourRecord("d6 home download badge", app)
        app.swipeDown()
        app.swipeDown()
        XCTAssertTrue(details.waitForExistence(timeout: 10), app.debugDescription)
        details.tap()
        XCTAssertTrue(app.buttons["Downloaded"].waitForExistence(timeout: 10), app.debugDescription)
        app.buttons["Downloaded"].tap()
        removeDownload(in: app)
        tourRecord("d7 downloads empty", app)
    }

    @MainActor private func removeDownload(in app: XCUIApplication) {
        let row = app.buttons.matching(NSPredicate(format: "label CONTAINS ' · '")).firstMatch
        XCTAssertTrue(row.waitForExistence(timeout: 10), app.debugDescription)
        row.swipeLeft()
        app.buttons["Remove"].firstMatch.tap()
        let confirm = app.buttons["Remove download"]
        XCTAssertTrue(confirm.waitForExistence(timeout: 5), app.debugDescription)
        confirm.tap()
        XCTAssertTrue(app.staticTexts["Take a title with you"].waitForExistence(timeout: 20), app.debugDescription)
    }


    @MainActor private func tour(_ app: XCUIApplication) async throws {
        let home = tourTab("Home", in: app)
        XCTAssertTrue(home.waitForExistence(timeout: 90), app.debugDescription)
        sleep(5)
        tourRecord("03 home top", app)
        app.swipeUp()
        sleep(2)
        tourRecord("04 home middle", app)
        app.swipeUp()
        app.swipeUp()
        sleep(2)
        tourRecord("05 home bottom", app)
        for (index, name) in ["Movies", "Shows"].enumerated() where tourTab(name, in: app).exists {
            tourTab(name, in: app).tap()
            sleep(4)
            tourRecord("0\(6 + index) \(name)", app)
        }
        if tourTab("Search", in: app).exists {
            tourTab("Search", in: app).tap()
            sleep(3)
            tourRecord("08 search empty", app)
            let field = app.searchFields.firstMatch
            if field.waitForExistence(timeout: 5) {
                field.tap()
                field.typeText("the")
                sleep(3)
                tourRecord("09 search results", app)
            }
            for label in ["Cancel", "Close"] where app.buttons[label].exists {
                app.buttons[label].firstMatch.tap()
                break
            }
            sleep(1)
        }
        tourReveal(home, in: app)
        home.tap()
        sleep(2)
        app.swipeDown()
        app.swipeDown()
        let details = app.buttons["Details"].firstMatch
        if details.waitForExistence(timeout: 5) {
            details.tap()
            sleep(4)
            tourRecord("10 detail", app)
            app.swipeUp()
            sleep(1)
            tourRecord("11 detail lower", app)
            app.swipeDown()
            let play = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Play' OR label BEGINSWITH 'Resume'")).firstMatch
            if play.waitForExistence(timeout: 3) {
                play.tap()
                sleep(6)
                tourRecord("12 playback", app)
                app.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.4)).tap()
                sleep(1)
                tourRecord("13 playback controls", app)
                if app.buttons["Close player"].exists { app.buttons["Close player"].tap() }
                sleep(2)
            }
        }
        tourReveal(home, in: app)
        if tourTab("More", in: app).exists {
            tourTab("More", in: app).tap()
            sleep(2)
            tourRecord("14 more", app)
        }
        tourReveal(home, in: app)
        home.tap()
        XCUIDevice.shared.orientation = .landscapeLeft
        sleep(3)
        tourRecord("15 home landscape", app)
        XCUIDevice.shared.orientation = .portrait
    }
}
