import XCTest

// Search uses the system keyboard above app-owned filters and results. The
// browser suite cannot detect directional gaps at that native boundary.
final class RemoteSearchNavigationTests: RemoteTestCase {
    @MainActor func testKeyboardResultsAndEmptyResultsKeepTheirReturnPaths() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        record("search initial keyboard", app)
        assertFocus("a", app)
        remote.press(.right)
        assertFocus("b", app)
        remote.press(.left)
        assertFocus("a", app)
        remote.press(.select)
        remote.press(.down)
        assertFocus("Sort: Title", app)
        remote.press(.down)
        XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains("A Quiet Evening") }))
        record("search first filtered result", app)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10))
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        XCTAssertTrue(focusedLabels(app).contains(where: { $0.contains("A Quiet Evening") }))
        remote.press(.up)
        assertFocus("Sort: Title", app)
        remote.press(.up)
        assertFocus("a", app)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["No matches"].waitForExistence(timeout: 10))
        record("search empty results", app)
        remote.press(.down)
        assertFocus("Sort: Title", app)
        remote.press(.up)
        assertFocus("a", app)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Search"].hasFocus)
    }

    @MainActor private func assertFocus(_ label: String, _ app: XCUIApplication,
                                        file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertTrue(focusedLabels(app).contains(label),
                      "Expected \(label); actual: \(focusedLabels(app))", file: file, line: line)
    }
}
