import XCTest

final class OtherServersJourneyTests: XCTestCase {
    private struct Fixture {
        let jellyfin: String
        let emby: String
        let user: String
        let password: String
    }

    @MainActor func testJellyfinAndEmbyShareOneLibraryAndPlay() async throws {
        let fixture = try fixture()
        continueAfterFailure = false
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        try signIn(app, address: fixture.jellyfin, kind: "Jellyfin", fixture: fixture, record: "j")
        let home = tourTab("Home", in: app)
        XCTAssertTrue(home.waitForExistence(timeout: 90), app.debugDescription)
        sleep(4)
        tourRecord("j3 home", app)

        tourTab("Movies", in: app).tap()
        let bunny = title("Big Buck Bunny", in: app)
        XCTAssertTrue(bunny.waitForExistence(timeout: 30), app.debugDescription)
        XCTAssertTrue(title("Sintel", in: app).exists, app.debugDescription)
        XCTAssertTrue(title("Tears of Steel", in: app).exists, app.debugDescription)
        sleep(2)
        tourRecord("j4 movies", app)
        bunny.tap()
        try play(app, record: "j5 direct playback")
        app.navigationBars.buttons.firstMatch.tap()
        title("Sintel", in: app).tap()
        try play(app, record: "j6 converted playback")
        app.navigationBars.buttons.firstMatch.tap()

        tourTab("Shows", in: app).tap()
        let show = title("Pioneer One", in: app)
        XCTAssertTrue(show.waitForExistence(timeout: 30), app.debugDescription)
        show.tap()
        XCTAssertTrue(app.buttons["Episode 1, Earthfall, next"].waitForExistence(timeout: 30), app.debugDescription)
        XCTAssertTrue(app.staticTexts["3 episodes"].exists, app.debugDescription)
        sleep(2)
        tourRecord("j7 show", app)
        app.navigationBars.buttons.firstMatch.tap()

        openSettings(app)
        XCTAssertTrue(app.staticTexts["Libraries"].waitForExistence(timeout: 10), app.debugDescription)
        app.buttons["Add a library"].tap()
        try signIn(app, address: fixture.emby, kind: "Emby", fixture: fixture, record: "e")
        openSettings(app)
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Emby ·'")).firstMatch.waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Jellyfin ·'")).firstMatch.exists, app.debugDescription)
        tourRecord("e3 libraries", app)

        tourTab("Movies", in: app).tap()
        XCTAssertTrue(title("Big Buck Bunny", in: app).waitForExistence(timeout: 30), app.debugDescription)
        sleep(3)
        XCTAssertEqual(app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Big Buck Bunny'")).count, 1, app.debugDescription)
        tourRecord("e4 combined movies", app)
    }

    @MainActor private func signIn(_ app: XCUIApplication, address: String, kind: String, fixture: Fixture, record: String) throws {
        let field = app.textFields["Server address"]
        XCTAssertTrue(field.waitForExistence(timeout: 20), app.debugDescription)
        tourRecord("\(record)1 setup", app)
        field.tap()
        app.typeText(address)
        app.buttons["Connect"].tap()
        let user = app.textFields["User name"]
        XCTAssertTrue(user.waitForExistence(timeout: 20), app.debugDescription)
        XCTAssertTrue(app.navigationBars[kind].exists, app.debugDescription)
        user.tap()
        app.typeText(fixture.user)
        app.secureTextFields["Password"].tap()
        app.typeText(fixture.password)
        tourRecord("\(record)2 sign in", app)
        app.buttons["Sign in"].tap()
        XCTAssertTrue(app.textFields["User name"].waitForNonExistence(timeout: 60), app.debugDescription)
    }

    @MainActor private func play(_ app: XCUIApplication, record: String) throws {
        let play = app.buttons["Play"].firstMatch
        XCTAssertTrue(play.waitForExistence(timeout: 30), app.debugDescription)
        play.tap()
        XCTAssertTrue(app.buttons["Close player"].waitForExistence(timeout: 15), app.debugDescription)
        XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 60), app.debugDescription)
        sleep(6)
        let close = app.buttons["Close player"]
        for _ in 0..<4 {
            app.coordinate(withNormalizedOffset: CGVector(dx: 0.75, dy: 0.3)).tap()
            sleep(1)
            if close.exists, close.frame.width > 0, close.isHittable { break }
        }
        tourRecord(record, app)
        close.tap()
        XCTAssertTrue(app.buttons["Details"].firstMatch.waitForExistence(timeout: 15) || app.buttons["Play"].firstMatch.waitForExistence(timeout: 5), app.debugDescription)
    }

    @MainActor private func openSettings(_ app: XCUIApplication) {
        let settings = tourTab("Settings", in: app)
        if settings.exists { settings.tap(); return }
        tourTab("More", in: app).tap()
        let link = app.buttons["Settings"].firstMatch
        XCTAssertTrue(link.waitForExistence(timeout: 10), app.debugDescription)
        link.tap()
    }

    @MainActor private func title(_ name: String, in app: XCUIApplication) -> XCUIElement {
        app.buttons.matching(NSPredicate(format: "label BEGINSWITH %@", name)).firstMatch
    }

    private func fixture() throws -> Fixture {
        let environment = ProcessInfo.processInfo.environment
        guard let jellyfin = environment["KINOSAIL_SOURCE_JELLYFIN"], !jellyfin.isEmpty,
              let emby = environment["KINOSAIL_SOURCE_EMBY"], !emby.isEmpty,
              let user = environment["KINOSAIL_SOURCE_USER"], let password = environment["KINOSAIL_SOURCE_PASSWORD"] else {
            throw XCTSkip("Set KINOSAIL_SOURCE_JELLYFIN, KINOSAIL_SOURCE_EMBY, KINOSAIL_SOURCE_USER and KINOSAIL_SOURCE_PASSWORD to populated test servers.")
        }
        return Fixture(jellyfin: jellyfin, emby: emby, user: user, password: password)
    }
}
