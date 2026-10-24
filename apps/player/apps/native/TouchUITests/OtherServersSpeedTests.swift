import XCTest

final class OtherServersSpeedTests: XCTestCase {
    @MainActor func testInstallToFirstFrameTimings() throws {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_SOURCE_JELLYFIN"], !server.isEmpty,
              let user = environment["KINOSAIL_SOURCE_USER"], let password = environment["KINOSAIL_SOURCE_PASSWORD"] else {
            throw XCTSkip("Set KINOSAIL_SOURCE_JELLYFIN, KINOSAIL_SOURCE_USER and KINOSAIL_SOURCE_PASSWORD to a populated Jellyfin test server.")
        }
        continueAfterFailure = false
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        var marks: [(String, Double)] = []
        var clock = Date()
        func mark(_ name: String) {
            marks.append((name, Date().timeIntervalSince(clock)))
            clock = Date()
        }

        app.launch()
        let field = app.textFields["Server address"]
        XCTAssertTrue(field.waitForExistence(timeout: 30), app.debugDescription)
        mark("launch to setup")
        field.tap()
        app.typeText(server)
        clock = Date()
        app.buttons["Connect"].tap()
        let secure = app.secureTextFields["Password"]
        XCTAssertTrue(secure.waitForExistence(timeout: 30), app.debugDescription)
        mark("connect to sign-in")
        if app.textFields["User name"].exists {
            app.textFields["User name"].tap()
            app.typeText(user)
        }
        secure.tap()
        app.typeText(password)
        clock = Date()
        app.buttons["Sign in"].tap()
        let start = app.buttons["Start watching"]
        XCTAssertTrue(start.waitForExistence(timeout: 60), app.debugDescription)
        mark("sign in to ready")
        XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'ready to watch'")).firstMatch.waitForExistence(timeout: 60), app.debugDescription)
        mark("ready to library preview")
        start.tap()
        let movie = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Big Buck Bunny' OR label BEGINSWITH 'Sintel' OR label BEGINSWITH 'Tears of Steel'")).firstMatch
        XCTAssertTrue(movie.waitForExistence(timeout: 90), app.debugDescription)
        mark("start to home titles")

        for name in ["Big Buck Bunny", "Sintel"] {
            tourTab("Movies", in: app).tap()
            let title = app.buttons.matching(NSPredicate(format: "label BEGINSWITH %@", name)).firstMatch
            XCTAssertTrue(title.waitForExistence(timeout: 30), app.debugDescription)
            clock = Date()
            title.tap()
            let play = app.buttons["Play"].firstMatch
            XCTAssertTrue(play.waitForExistence(timeout: 30), app.debugDescription)
            mark("\(name) detail")
            play.tap()
            XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 60), app.debugDescription)
            mark("\(name) play to playing")
            sleep(2)
            let close = app.buttons["Close player"]
            for _ in 0..<4 {
                app.coordinate(withNormalizedOffset: CGVector(dx: 0.75, dy: 0.3)).tap()
                sleep(1)
                if close.exists, close.isHittable { break }
            }
            close.tap()
            XCTAssertTrue(play.waitForExistence(timeout: 15), app.debugDescription)
            app.navigationBars.buttons.firstMatch.tap()
        }

        let report = marks.map { String(format: "%@=%.2fs", $0.0, $0.1) }.joined(separator: "\n")
        print("FUNNEL\n" + report)
        let attachment = XCTAttachment(string: report)
        attachment.name = "funnel timings"
        attachment.lifetime = .keepAlways
        add(attachment)
    }
}
