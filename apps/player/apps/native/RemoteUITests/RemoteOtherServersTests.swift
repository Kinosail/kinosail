import XCTest

final class RemoteOtherServersTests: RemoteTestCase {
    @MainActor func testJellyfinQuickConnectOnboarding() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_SOURCE_JELLYFIN"], !server.isEmpty,
              let user = environment["KINOSAIL_SOURCE_USER"], let password = environment["KINOSAIL_SOURCE_PASSWORD"] else {
            throw XCTSkip("Set KINOSAIL_SOURCE_JELLYFIN, KINOSAIL_SOURCE_USER and KINOSAIL_SOURCE_PASSWORD to a populated Jellyfin test server.")
        }
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        let field = app.textFields["Server address"]
        XCTAssertTrue(field.waitForExistence(timeout: 20), app.debugDescription)
        sleep(4)
        record("tv1 setup", app)
        for _ in 0..<8 where !field.hasFocus { remote.press(.down) }
        XCTAssertTrue(field.hasFocus, app.debugDescription)
        remote.press(.select)
        sleep(2)
        app.typeText(server + "\n")

        let code = app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Quick Connect code'")).firstMatch
        XCTAssertTrue(code.waitForExistence(timeout: 30), app.debugDescription)
        sleep(2)
        record("tv2 sign in", app)
        try await approve(code.label.filter(\.isNumber), server: server, user: user, password: password)

        let start = app.buttons["Start watching"]
        XCTAssertTrue(start.waitForExistence(timeout: 30), app.debugDescription)
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'ready to watch'")).firstMatch.waitForExistence(timeout: 30), app.debugDescription)
        sleep(2)
        record("tv3 ready", app)
        for _ in 0..<4 where !start.hasFocus { remote.press(.up) }
        XCTAssertTrue(start.hasFocus, app.debugDescription)
        remote.press(.select)
        XCTAssertTrue(start.waitForNonExistence(timeout: 60), app.debugDescription)
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Big Buck Bunny' OR label BEGINSWITH 'Sintel' OR label BEGINSWITH 'Tears of Steel'")).firstMatch.waitForExistence(timeout: 60), app.debugDescription)
        sleep(4)
        record("tv4 home", app)
    }

    @MainActor func testPlexCodeOnboarding() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let accounts = environment["KINOSAIL_PLEX_TV"], !accounts.isEmpty else {
            throw XCTSkip("Set KINOSAIL_PLEX_TV to the plex.tv stand-in from scripts/other-servers-fixture/up.sh.")
        }
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launchEnvironment["KINOSAIL_PLEX_TV"] = accounts
        let remote = XCUIRemote.shared
        app.launch()
        let entry = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Sign in with Plex'")).firstMatch
        XCTAssertTrue(entry.waitForExistence(timeout: 20), app.debugDescription)
        sleep(4)
        for _ in 0..<10 where !entry.hasFocus { remote.press(.down) }
        XCTAssertTrue(entry.hasFocus, app.debugDescription)
        record("tvp1 setup", app)
        remote.press(.select)

        let code = app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Plex code '")).firstMatch
        XCTAssertTrue(code.waitForExistence(timeout: 30), app.debugDescription)
        sleep(2)
        record("tvp2 plex code", app)
        let value = String(code.label.dropFirst("Plex code ".count)).replacingOccurrences(of: " ", with: "")
        var link = URLRequest(url: try XCTUnwrap(URL(string: "\(accounts)/link?code=\(value)")))
        link.httpMethod = "POST"
        let (_, response) = try await URLSession.shared.data(for: link)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)

        let start = app.buttons["Start watching"]
        XCTAssertTrue(start.waitForExistence(timeout: 60), app.debugDescription)
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'ready to watch'")).firstMatch.waitForExistence(timeout: 30), app.debugDescription)
        sleep(2)
        record("tvp3 ready", app)
        for _ in 0..<4 where !start.hasFocus { remote.press(.up) }
        XCTAssertTrue(start.hasFocus, app.debugDescription)
        remote.press(.select)
        XCTAssertTrue(start.waitForNonExistence(timeout: 60), app.debugDescription)
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Big Buck Bunny' OR label BEGINSWITH 'Sintel' OR label BEGINSWITH 'Tears of Steel'")).firstMatch.waitForExistence(timeout: 60), app.debugDescription)
        sleep(4)
        record("tvp4 home", app)
    }

    private func approve(_ code: String, server: String, user: String, password: String) async throws {
        let header = "MediaBrowser Client=\"Kinosail QA\", Device=\"QA\", DeviceId=\"kinosail-remote-qa\", Version=\"1\""
        var login = URLRequest(url: try XCTUnwrap(URL(string: "\(server)/Users/AuthenticateByName")))
        login.httpMethod = "POST"
        login.setValue(header, forHTTPHeaderField: "X-Emby-Authorization")
        login.setValue("application/json", forHTTPHeaderField: "Content-Type")
        login.httpBody = try JSONSerialization.data(withJSONObject: ["Username": user, "Pw": password])
        let (data, _) = try await URLSession.shared.data(for: login)
        let token = try XCTUnwrap((try JSONSerialization.jsonObject(with: data) as? [String: Any])?["AccessToken"] as? String)
        var authorize = URLRequest(url: try XCTUnwrap(URL(string: "\(server)/QuickConnect/Authorize?code=\(code)")))
        authorize.httpMethod = "POST"
        authorize.setValue(header + ", Token=\"\(token)\"", forHTTPHeaderField: "X-Emby-Authorization")
        let (_, response) = try await URLSession.shared.data(for: authorize)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
    }
}
