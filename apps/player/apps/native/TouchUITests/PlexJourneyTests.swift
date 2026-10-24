import XCTest

final class PlexJourneyTests: XCTestCase {
    @MainActor func testPlexSignInBrowsesPlaysAndJoinsJellyfin() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let accounts = environment["KINOSAIL_PLEX_TV"], !accounts.isEmpty, let plex = environment["KINOSAIL_SOURCE_PLEX"], !plex.isEmpty else {
            throw XCTSkip("Set KINOSAIL_PLEX_TV and KINOSAIL_SOURCE_PLEX to the Plex fixture from scripts/other-servers-fixture/up.sh.")
        }
        continueAfterFailure = false
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launchEnvironment["KINOSAIL_PLEX_TV"] = accounts
        app.launch()

        let entry = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Sign in with Plex'")).firstMatch
        XCTAssertTrue(entry.waitForExistence(timeout: 20), app.debugDescription)
        sleep(2)
        tourRecord("p1 setup", app)
        entry.tap()
        let web = app.buttons["Continue with Plex"]
        XCTAssertTrue(web.waitForExistence(timeout: 20), app.debugDescription)
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Plex code '")).firstMatch.exists, app.debugDescription)
        tourRecord("p2 plex sign in", app)
        web.tap()
        let springboard = XCUIApplication(bundleIdentifier: "com.apple.springboard")
        if springboard.buttons["Continue"].waitForExistence(timeout: 10) { springboard.buttons["Continue"].tap() }

        let start = app.buttons["Start watching"]
        XCTAssertTrue(start.waitForExistence(timeout: 60), app.debugDescription)
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'ready to watch'")).firstMatch.waitForExistence(timeout: 30), app.debugDescription)
        sleep(2)
        tourRecord("p3 ready", app)
        start.tap()
        XCTAssertTrue(tourTab("Home", in: app).waitForExistence(timeout: 90), app.debugDescription)
        sleep(3)
        tourRecord("p4 home", app)

        tourTab("Movies", in: app).tap()
        let bunny = title("Big Buck Bunny", in: app)
        XCTAssertTrue(bunny.waitForExistence(timeout: 30), app.debugDescription)
        XCTAssertTrue(title("Sintel", in: app).exists && title("Tears of Steel", in: app).exists, app.debugDescription)
        sleep(2)
        tourRecord("p5 movies", app)
        bunny.tap()
        try play(app, record: "p6 direct playback with subtitles", subtitle: "English")
        app.navigationBars.buttons.firstMatch.tap()
        title("Sintel", in: app).tap()
        try play(app, record: "p6b hevc mkv playback", conversionOn: plex)
        app.navigationBars.buttons.firstMatch.tap()
        title("Tears of Steel", in: app).tap()
        try play(app, record: "p7 converted playback", conversionOn: plex)
        app.navigationBars.buttons.firstMatch.tap()

        tourTab("Shows", in: app).tap()
        let show = title("Pioneer One", in: app)
        XCTAssertTrue(show.waitForExistence(timeout: 30), app.debugDescription)
        show.tap()
        XCTAssertTrue(app.staticTexts["3 episodes"].waitForExistence(timeout: 30), app.debugDescription)
        sleep(2)
        tourRecord("p8 show", app)
        app.navigationBars.buttons.firstMatch.tap()

        guard let jellyfin = environment["KINOSAIL_SOURCE_JELLYFIN"], let user = environment["KINOSAIL_SOURCE_USER"],
              let password = environment["KINOSAIL_SOURCE_PASSWORD"] else { return }
        openSettings(app)
        XCTAssertTrue(app.staticTexts["Plex · kino"].waitForExistence(timeout: 10), app.debugDescription)
        app.buttons["Add a server"].tap()
        let field = app.textFields["Server address"]
        XCTAssertTrue(field.waitForExistence(timeout: 20), app.debugDescription)
        _ = app.buttons["Search again"].waitForExistence(timeout: 30)
        field.tap()
        for _ in 0..<3 where !(field.value(forKey: "hasKeyboardFocus") as? Bool ?? false) {
            sleep(1)
            field.tap()
        }
        app.typeText(jellyfin)
        app.buttons["Connect"].tap()
        let secure = app.secureTextFields["Password"]
        XCTAssertTrue(secure.waitForExistence(timeout: 20), app.debugDescription)
        if app.textFields["User name"].exists { app.textFields["User name"].tap(); app.typeText(user) }
        secure.tap()
        app.typeText(password)
        app.buttons["Sign in"].tap()
        XCTAssertTrue(start.waitForExistence(timeout: 60), app.debugDescription)
        start.tap()
        XCTAssertTrue(start.waitForNonExistence(timeout: 60), app.debugDescription)
        tourTab("Movies", in: app).tap()
        XCTAssertTrue(title("Big Buck Bunny", in: app).waitForExistence(timeout: 30), app.debugDescription)
        sleep(3)
        XCTAssertEqual(app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Big Buck Bunny'")).count, 1, app.debugDescription)
        tourRecord("p9 plex and jellyfin together", app)

        openSettings(app)
        let signOut = app.buttons["Sign out"].firstMatch
        for _ in 0..<8 where !signOut.isHittable { app.swipeUp() }
        signOut.tap()
        XCTAssertTrue(app.alerts.buttons["Sign out"].waitForExistence(timeout: 10), app.debugDescription)
        app.alerts.buttons["Sign out"].tap()
        XCTAssertTrue(app.staticTexts["From your Apple devices"].waitForExistence(timeout: 30), app.debugDescription)
        app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Kino Plex, Plex, signed in as'")).firstMatch.tap()
        XCTAssertTrue(start.waitForExistence(timeout: 30), app.debugDescription)
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'ready to watch'")).firstMatch.waitForExistence(timeout: 30), app.debugDescription)
        sleep(1)
        tourRecord("p10 plex from apple devices", app)
    }

    @MainActor private func play(_ app: XCUIApplication, record: String, conversionOn plex: String? = nil, subtitle: String? = nil) throws {
        let play = app.buttons["Play"].firstMatch
        XCTAssertTrue(play.waitForExistence(timeout: 30), app.debugDescription)
        sleep(2)
        play.tap()
        XCTAssertTrue(app.buttons["Close player"].waitForExistence(timeout: 15), app.debugDescription)
        XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 60), app.debugDescription)
        sleep(5)
        if let plex {
            let done = expectation(description: "Plex conversion session")
            Task.detached {
                let sessions = try? await Self.transcodes(plex)
                XCTAssertGreaterThan(sessions ?? 0, 0, "Plex shows no conversion session while converted playback runs")
                done.fulfill()
            }
            wait(for: [done], timeout: 30)
        }
        let close = app.buttons["Close player"]
        if let subtitle {
            reveal(close, in: app)
            let tracks = app.buttons.matching(NSPredicate(format: "label == 'Audio & subtitles' OR label == 'Subtitles'")).firstMatch
            XCTAssertTrue(tracks.waitForExistence(timeout: 10), app.debugDescription)
            tracks.tap()
            let choice = app.buttons.matching(NSPredicate(format: "label BEGINSWITH %@", subtitle)).firstMatch
            XCTAssertTrue(choice.waitForExistence(timeout: 10), app.debugDescription)
            tourRecord(record + " tracks", app)
            choice.tap()
            sleep(1)
            app.buttons["Done"].firstMatch.tap()
            XCTAssertTrue(app.staticTexts["Kinosail test subtitle"].waitForExistence(timeout: 15), app.debugDescription)
        }
        reveal(close, in: app)
        tourRecord(record, app)
        close.tap()
        if !app.buttons["Play"].firstMatch.waitForExistence(timeout: 5), close.exists, close.isHittable { close.tap() }
        XCTAssertTrue(app.buttons["Play"].firstMatch.waitForExistence(timeout: 15), app.debugDescription)
    }

    @MainActor private func reveal(_ close: XCUIElement, in app: XCUIApplication) {
        for _ in 0..<4 {
            app.coordinate(withNormalizedOffset: CGVector(dx: 0.75, dy: 0.3)).tap()
            sleep(1)
            if close.exists, close.isHittable { break }
        }
    }

    private static func transcodes(_ plex: String) async throws -> Int {
        guard let url = URL(string: plex + "/transcode/sessions") else { return 0 }
        var request = URLRequest(url: url)
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        let (data, _) = try await URLSession.shared.data(for: request)
        let value = try JSONSerialization.jsonObject(with: data) as? [String: Any]
        return (value?["MediaContainer"] as? [String: Any])?["size"] as? Int ?? 0
    }

    @MainActor private func openSettings(_ app: XCUIApplication) {
        let settings = tourTab("Settings", in: app)
        if settings.exists { settings.tap(); return }
        tourTab("More", in: app).tap()
        if app.staticTexts["Libraries"].waitForExistence(timeout: 2) { return }
        let link = app.buttons["Settings"].firstMatch
        XCTAssertTrue(link.waitForExistence(timeout: 10), app.debugDescription)
        link.tap()
    }

    @MainActor private func title(_ name: String, in app: XCUIApplication) -> XCUIElement {
        app.buttons.matching(NSPredicate(format: "label BEGINSWITH %@", name)).firstMatch
    }
}
