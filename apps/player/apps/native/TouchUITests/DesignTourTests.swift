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
        let address = app.textFields["Server address"]
        if address.waitForExistence(timeout: 5) {
            record("01 setup", app)
            address.tap()
            for _ in 0..<3 where !(address.value(forKey: "hasKeyboardFocus") as? Bool ?? false) {
                sleep(1)
                address.tap()
            }
            app.typeText(server)
            app.buttons["Connect"].tap()
            let code = app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Approval code'")).firstMatch
            XCTAssertTrue(code.waitForExistence(timeout: 20), app.debugDescription)
            record("02 pairing", app)
            try await approve(code.label.filter(\.isNumber), server: server,
                              token: environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? "")
        }
        let home = tab("Home", in: app)
        XCTAssertTrue(home.waitForExistence(timeout: 90), app.debugDescription)
        sleep(5)
        record("03 home top", app)
        app.swipeUp()
        sleep(2)
        record("04 home middle", app)
        app.swipeUp()
        app.swipeUp()
        sleep(2)
        record("05 home bottom", app)
        for (index, name) in ["Movies", "Shows"].enumerated() where tab(name, in: app).exists {
            tab(name, in: app).tap()
            sleep(4)
            record("0\(6 + index) \(name)", app)
        }
        if tab("Search", in: app).exists {
            tab("Search", in: app).tap()
            sleep(3)
            record("08 search empty", app)
            let field = app.searchFields.firstMatch
            if field.waitForExistence(timeout: 5) {
                field.tap()
                field.typeText("the")
                sleep(3)
                record("09 search results", app)
            }
            for label in ["Cancel", "Close"] where app.buttons[label].exists {
                app.buttons[label].firstMatch.tap()
                break
            }
            sleep(1)
        }
        reveal(home, in: app)
        home.tap()
        sleep(2)
        app.swipeDown()
        app.swipeDown()
        let details = app.buttons["Details"].firstMatch
        if details.waitForExistence(timeout: 5) {
            details.tap()
            sleep(4)
            record("10 detail", app)
            app.swipeUp()
            sleep(1)
            record("11 detail lower", app)
            app.swipeDown()
            let play = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Play' OR label BEGINSWITH 'Resume'")).firstMatch
            if play.waitForExistence(timeout: 3) {
                play.tap()
                sleep(6)
                record("12 playback", app)
                app.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.4)).tap()
                sleep(1)
                record("13 playback controls", app)
                if app.buttons["Close player"].exists { app.buttons["Close player"].tap() }
                sleep(2)
            }
        }
        reveal(home, in: app)
        if tab("More", in: app).exists {
            tab("More", in: app).tap()
            sleep(2)
            record("14 more", app)
        }
        reveal(home, in: app)
        home.tap()
        XCUIDevice.shared.orientation = .landscapeLeft
        sleep(3)
        record("15 home landscape", app)
        XCUIDevice.shared.orientation = .portrait
    }

    @MainActor private func tab(_ name: String, in app: XCUIApplication) -> XCUIElement {
        let bar = app.tabBars.buttons[name]
        return bar.exists ? bar : app.buttons.matching(identifier: name).firstMatch
    }

    @MainActor private func reveal(_ tab: XCUIElement, in app: XCUIApplication) {
        for _ in 0..<3 where !tab.isHittable { app.swipeDown() }
    }

    private func approve(_ code: String, server: String, token: String) async throws {
        XCTAssertEqual(code.count, 6)
        var request = URLRequest(url: try XCTUnwrap(URL(string: "\(server)/api/v1/quick-connect/\(code)")))
        request.httpMethod = "POST"
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        let (_, response) = try await URLSession.shared.data(for: request)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 204)
    }

    @MainActor private func record(_ name: String, _ app: XCUIApplication) {
        let attachment = XCTAttachment(screenshot: XCUIScreen.main.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }
}
