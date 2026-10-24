import XCTest

final class OtherServersHandoffTests: XCTestCase {
    @MainActor func testPhoneApprovesTVAndSharedServerReturnsWithoutPassword() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_SOURCE_JELLYFIN"], !server.isEmpty,
              let user = environment["KINOSAIL_SOURCE_USER"], let password = environment["KINOSAIL_SOURCE_PASSWORD"] else {
            throw XCTSkip("Set KINOSAIL_SOURCE_JELLYFIN, KINOSAIL_SOURCE_USER and KINOSAIL_SOURCE_PASSWORD to a populated Jellyfin test server.")
        }
        continueAfterFailure = false
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()

        let field = app.textFields["Server address"]
        XCTAssertTrue(field.waitForExistence(timeout: 20), app.debugDescription)
        field.tap()
        app.typeText(server)
        app.buttons["Connect"].tap()
        let secure = app.secureTextFields["Password"]
        XCTAssertTrue(secure.waitForExistence(timeout: 20), app.debugDescription)
        if app.textFields["User name"].exists {
            app.textFields["User name"].tap()
            app.typeText(user)
        }
        secure.tap()
        app.typeText(password)
        app.buttons["Sign in"].tap()
        let start = app.buttons["Start watching"]
        XCTAssertTrue(start.waitForExistence(timeout: 60), app.debugDescription)
        start.tap()
        XCTAssertTrue(tourTab("Home", in: app).waitForExistence(timeout: 90), app.debugDescription)

        let tv = try await QuickConnectFixture.initiate(server)
        let link = try XCTUnwrap(URL(string: "kinosail://quickconnect?server=\(tv.serverID)&code=\(tv.code)"))
        app.open(link)
        let springboard = XCUIApplication(bundleIdentifier: "com.apple.springboard")
        if springboard.buttons["Open"].waitForExistence(timeout: 5) { springboard.buttons["Open"].tap() }
        sleep(2)
        tourRecord("h0 after link", app)
        XCTAssertTrue(app.staticTexts["Sign in your TV?"].waitForExistence(timeout: 20), app.debugDescription)
        XCTAssertTrue(app.staticTexts["Code \(tv.code.map(String.init).joined(separator: " "))"].exists, app.debugDescription)
        sleep(1)
        tourRecord("h1 approve tv", app)
        let before = try await QuickConnectFixture.authenticated(server, secret: tv.secret)
        XCTAssertFalse(before)
        app.buttons["Approve"].tap()
        XCTAssertTrue(app.staticTexts["Your TV is signed in"].waitForExistence(timeout: 20), app.debugDescription)
        let after = try await QuickConnectFixture.authenticated(server, secret: tv.secret)
        XCTAssertTrue(after)
        sleep(1)
        tourRecord("h2 tv approved", app)
        app.buttons["Done"].tap()

        openSettings(app)
        let signOut = app.buttons["Sign out"].firstMatch
        for _ in 0..<8 where !signOut.isHittable { app.swipeUp() }
        signOut.tap()
        XCTAssertTrue(app.alerts.buttons["Sign out"].waitForExistence(timeout: 10), app.debugDescription)
        app.alerts.buttons["Sign out"].tap()

        let shared = app.staticTexts["From your Apple devices"]
        XCTAssertTrue(shared.waitForExistence(timeout: 30), app.debugDescription)
        sleep(2)
        tourRecord("h3 shared servers", app)
        app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Jellyfin, Jellyfin, signed in as'")).firstMatch.tap()
        XCTAssertTrue(start.waitForExistence(timeout: 30), app.debugDescription)
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'ready to watch'")).firstMatch.waitForExistence(timeout: 30), app.debugDescription)
        sleep(1)
        tourRecord("h4 ready without password", app)
        start.tap()
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Big Buck Bunny' OR label BEGINSWITH 'Sintel' OR label BEGINSWITH 'Tears of Steel'")).firstMatch.waitForExistence(timeout: 60), app.debugDescription)
        tourRecord("h5 home", app)
    }

    @MainActor private func openSettings(_ app: XCUIApplication) {
        let settings = tourTab("Settings", in: app)
        if settings.exists { settings.tap(); return }
        tourTab("More", in: app).tap()
        let link = app.buttons["Settings"].firstMatch
        XCTAssertTrue(link.waitForExistence(timeout: 10), app.debugDescription)
        link.tap()
    }
}

private enum QuickConnectFixture {
    struct Pending { let serverID: String; let code: String; let secret: String }
    private static let header = "MediaBrowser Client=\"Kinosail QA TV\", Device=\"QA TV\", DeviceId=\"kinosail-handoff-qa\", Version=\"1\""

    static func initiate(_ server: String) async throws -> Pending {
        let info = try await json(URLRequest(url: try XCTUnwrap(URL(string: "\(server)/System/Info/Public"))))
        var request = URLRequest(url: try XCTUnwrap(URL(string: "\(server)/QuickConnect/Initiate")))
        request.httpMethod = "POST"
        request.setValue(header, forHTTPHeaderField: "X-Emby-Authorization")
        let value = try await json(request)
        return Pending(serverID: try XCTUnwrap(info["Id"] as? String), code: try XCTUnwrap(value["Code"] as? String),
                       secret: try XCTUnwrap(value["Secret"] as? String))
    }

    static func authenticated(_ server: String, secret: String) async throws -> Bool {
        var request = URLRequest(url: try XCTUnwrap(URL(string: "\(server)/QuickConnect/Connect?Secret=\(secret)")))
        request.setValue(header, forHTTPHeaderField: "X-Emby-Authorization")
        return try await json(request)["Authenticated"] as? Bool == true
    }

    private static func json(_ request: URLRequest) async throws -> [String: Any] {
        let (data, _) = try await URLSession.shared.data(for: request)
        return try XCTUnwrap(try JSONSerialization.jsonObject(with: data) as? [String: Any])
    }
}
