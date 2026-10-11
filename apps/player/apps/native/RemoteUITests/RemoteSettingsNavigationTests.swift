import XCTest

// The browser suite cannot detect skipped or unreachable native Form rows.
// Existing remote tests stop at Playback; this owns the remaining Settings path.
final class RemoteSettingsNavigationTests: RemoteTestCase {
    @MainActor func testNestedSettingsReturnToTheirOriginatingRows() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        remote.press(.right)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Change Server"].waitForExistence(timeout: 10))
        let rows = ["Change Server", "Connect a TV", "Playback", "Progress sync",
                    "Show titles on Apple TV Home", "Supporter collection",
                    "Made possible by", "Privacy policy"]
        for (index, row) in rows.enumerated() {
            if index > 0 { remote.press(.down) }
            XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains(row) }))
            if row == "Playback" || row == "Show titles on Apple TV Home" { continue }
            remote.press(.select)
            switch row {
            case "Change Server":
                XCTAssertTrue(app.textFields["Server address"].waitForExistence(timeout: 10))
            case "Connect a TV":
                XCTAssertTrue(app.textFields["Six-digit code"].waitForExistence(timeout: 10))
            case "Progress sync":
                XCTAssertTrue(app.staticTexts["Progress sync"].waitForExistence(timeout: 10))
            case "Supporter collection":
                XCTAssertTrue(app.buttons["Back to Settings"].waitForExistence(timeout: 10))
                XCTAssertTrue(app.buttons["Try again"].waitForExistence(timeout: 10),
                              "The navigation fixture deliberately does not supply supporter state")
                remote.press(.down)
                XCTAssertTrue(app.buttons["Try again"].hasFocus)
                remote.press(.up)
                XCTAssertTrue(app.buttons["Back to Settings"].hasFocus)
                record("supporter failure return", app)
            case "Made possible by":
                XCTAssertTrue(app.staticTexts["Thank you to the people who make private playback possible."]
                    .waitForExistence(timeout: 10))
            case "Privacy policy":
                XCTAssertTrue(app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Your library stays'"))
                    .firstMatch.waitForExistence(timeout: 10))
            default: XCTFail("Unhandled Settings destination")
            }
            remote.press(.menu)
            XCTAssertTrue(app.buttons["Change Server"].waitForExistence(timeout: 10))
            XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains(row) }),
                          "Back should restore \(row); actual: \(focusedLabels(app))")
        }
    }

    @MainActor func testEverySettingsRowIsOneArrowAwayAndSignOutCanBeCancelled() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        remote.press(.right)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Change Server"].waitForExistence(timeout: 10))
        let rows = ["Change Server", "Connect a TV", "Playback", "Progress sync",
                    "Show titles on Apple TV Home", "Supporter collection",
                    "Made possible by", "Privacy policy", "Sign out"]
        for (index, row) in rows.enumerated() {
            if index > 0 { remote.press(.down) }
            XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains(row) }),
                          "One Down should focus \(row); actual: \(focusedLabels(app))")
            record("settings down to \(row)", app)
        }
        remote.press(.select)
        XCTAssertTrue(app.alerts["Sign out of this device?"].waitForExistence(timeout: 5))
        remote.press(.menu)
        XCTAssertTrue(app.alerts["Sign out of this device?"].waitForNonExistence(timeout: 5))
        XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains("Sign out") }))
        for row in rows.dropLast().reversed() {
            remote.press(.up)
            XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains(row) }),
                          "One Up should focus \(row); actual: \(focusedLabels(app))")
        }
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Settings"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Settings"].hasFocus)
    }
}
