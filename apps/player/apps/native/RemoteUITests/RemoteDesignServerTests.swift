import XCTest

final class RemoteDesignServerTests: RemoteTestCase {
    @MainActor func testConnectToDesignServer() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_DESIGN_TOUR_SERVER"], !server.isEmpty else {
            throw XCTSkip("Set KINOSAIL_DESIGN_TOUR_SERVER to a populated test Server.")
        }
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        let address = app.textFields["Server address"]
        guard address.waitForExistence(timeout: 10) else { return }
        for _ in 0..<6 where !address.hasFocus { XCUIRemote.shared.press(.up); usleep(400_000) }
        XCUIRemote.shared.press(.select)
        sleep(2)
        app.typeText(server + "\n")
        sleep(2)
        let code = app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Approval code'")).firstMatch
        if !code.waitForExistence(timeout: 10) {
            for _ in 0..<4 where !app.buttons["Connect"].hasFocus { XCUIRemote.shared.press(.down); usleep(400_000) }
            XCUIRemote.shared.press(.select)
        }
        XCTAssertTrue(code.waitForExistence(timeout: 20), app.debugDescription)
        record("tv-pairing", app)
        var request = URLRequest(url: try XCTUnwrap(URL(string: "\(server)/api/v1/quick-connect/\(code.label.filter(\.isNumber))")))
        request.httpMethod = "POST"
        request.setValue("Bearer \(environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? "")", forHTTPHeaderField: "Authorization")
        let (_, response) = try await URLSession.shared.data(for: request)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 204)
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 60) || app.buttons["Search"].waitForExistence(timeout: 30), app.debugDescription)
    }
}
