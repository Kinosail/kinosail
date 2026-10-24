import XCTest

extension XCTestCase {
    @MainActor func tourConnect(_ app: XCUIApplication, server: String, token: String, records: Bool) async throws {
        let address = app.textFields["Server address"]
        if address.waitForExistence(timeout: 5) {
            if records { tourRecord("01 setup", app) }
            address.tap()
            for _ in 0..<3 where !(address.value(forKey: "hasKeyboardFocus") as? Bool ?? false) {
                sleep(1)
                address.tap()
            }
            app.typeText(server)
            app.buttons["Connect"].tap()
            let code = app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Approval code'")).firstMatch
            XCTAssertTrue(code.waitForExistence(timeout: 20), app.debugDescription)
            if records { tourRecord("02 pairing", app) }
            try await tourApprove(code.label.filter(\.isNumber), server: server, token: token)
        }
    }

    @MainActor func tourTab(_ name: String, in app: XCUIApplication) -> XCUIElement {
        let bar = app.tabBars.buttons[name]
        return bar.exists ? bar : app.buttons.matching(identifier: name).firstMatch
    }

    @MainActor func tourReveal(_ tab: XCUIElement, in app: XCUIApplication) {
        for _ in 0..<3 where !tab.isHittable { app.swipeDown() }
    }

    func tourApprove(_ code: String, server: String, token: String) async throws {
        XCTAssertEqual(code.count, 6)
        var request = URLRequest(url: try XCTUnwrap(URL(string: "\(server)/api/v1/quick-connect/\(code)")))
        request.httpMethod = "POST"
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        let (_, response) = try await URLSession.shared.data(for: request)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 204)
    }

    @MainActor func tourRecord(_ name: String, _ app: XCUIApplication) {
        let attachment = XCTAttachment(screenshot: XCUIScreen.main.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }
}
