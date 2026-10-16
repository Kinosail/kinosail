import UIKit
import XCTest

final class RemoteLibraryNavigationTests: RemoteTestCase {
    @MainActor func testGridColumnsAndTopRowReachSortWithOneUp() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        openMovies(app)
        if !focusedLabels(app).contains(where: { $0.contains("After the Rain") }) { remote.press(.down) }
        for title in ["An Extraordinary Journey", "Before Dawn", "Journey 006", "Journey 007", "Journey 008"] {
            remote.press(.right)
            assertFocus(title, app)
        }
        record("library top right card", app)
        remote.press(.down)
        assertFocus("Journey 014", app)
        for title in ["Journey 013", "Journey 012", "Journey 011", "Journey 010", "Journey 009"] {
            remote.press(.left)
            assertFocus(title, app)
        }
        remote.press(.up)
        assertFocus("After the Rain", app)
        for _ in 0..<5 { remote.press(.right) }
        assertFocus("Journey 008", app)
        remote.press(.up)
        record("library top right Up to Sort", app)
        assertFocus("Sort: Title", app)
        remote.press(.select)
        XCTAssertTrue(app.descendants(matching: .any)["Recently added"].waitForExistence(timeout: 5))
        remote.press(.down)
        record("library sort Down to Recently added", app)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Recently added"].waitForExistence(timeout: 5))
        assertFocus("Sort: Recently added", app)
        remote.press(.select)
        remote.press(.down)
        remote.press(.down)
        record("library sort Down to Year", app)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Year"].waitForExistence(timeout: 5))
        assertFocus("Sort: Year", app)
        remote.press(.select)
        record("library sort reopens on Title", app)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 5))
        remote.press(.select)
        XCTAssertTrue(app.descendants(matching: .any)["Recently added"].waitForExistence(timeout: 5))
        remote.press(.menu)
        record("library Sort after cancelling menu", app)
        assertFocus("Sort: Title", app)
    }

    @MainActor func testLetterIndexSkipsPagesAndReturnsToAdjacentTitle() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        openMovies(app)
        if !focusedLabels(app).contains(where: { $0.contains("Sort:") }) { remote.press(.up) }
        remote.press(.right)
        assertFocus("A, 2 titles", app)
        remote.press(.down)
        assertFocus("B, 1 title", app)
        remote.press(.down)
        assertFocus("J, 154 titles", app)
        record("library J index", app)
        assertReadableLetter(app.buttons["J, 154 titles"], app)
        for letter in ["M", "N", "T"] {
            remote.press(.down)
            assertFocus("\(letter), 1 title", app)
        }
        for letter in ["N", "M", "J"] {
            remote.press(.up)
            assertFocus("\(letter),", app)
        }
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "label CONTAINS 'Journey 006'"))
            .firstMatch.waitForExistence(timeout: 10))
        remote.press(.left)
        assertFocus("Journey", app)
        let title = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
        XCTAssertTrue(title.isHittable)
        XCTAssertGreaterThan(title.frame.midX, app.frame.width / 2,
                             "Left from the index should reach the adjacent right side of the grid")
        let label = title.label
        record("library left from J to title", app)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10))
        remote.press(.menu)
        XCTAssertTrue(app.buttons[label].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons[label].hasFocus, "Back should keep the letter jump and originating title")
    }

    @MainActor private func assertReadableLetter(_ letter: XCUIElement, _ app: XCUIApplication) {
        let readable = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            guard let screenshot = app.screenshot().image.cgImage else { return false }
            let scale = CGFloat(screenshot.width) / app.frame.width
            let frame = letter.frame
            let center = frame.insetBy(dx: frame.width * 0.3, dy: frame.height * 0.25)
            guard let glyph = screenshot.cropping(to: CGRect(
                x: center.minX * scale, y: center.minY * scale,
                width: center.width * scale, height: center.height * scale)),
                  let context = CGContext(data: nil, width: glyph.width, height: glyph.height,
                    bitsPerComponent: 8, bytesPerRow: glyph.width * 4,
                    space: CGColorSpaceCreateDeviceRGB(),
                    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue),
                  let pixels = context.data?.assumingMemoryBound(to: UInt8.self) else { return false }
            context.draw(glyph, in: CGRect(x: 0, y: 0, width: glyph.width, height: glyph.height))
            let dark = (0..<(glyph.width * glyph.height)).filter { index in
                (0..<3).allSatisfy { pixels[index * 4 + $0] < 100 }
            }.count
            return dark > 12
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [readable], timeout: 5), .completed,
                       "The focused letter must have a readable dark glyph inside its light fill")
    }

    @MainActor private func openMovies(_ app: XCUIApplication) {
        launchAtSearch(app)
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        XCUIRemote.shared.press(.down)
        XCUIRemote.shared.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        XCUIRemote.shared.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["A, 2 titles"].waitForExistence(timeout: 10), "Use the fixture with its populated A–Z index")
    }

    @MainActor private func assertFocus(_ title: String, _ app: XCUIApplication,
                                        file: StaticString = #filePath, line: UInt = #line) {
        let focused = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            self.focusedLabels(app).contains(where: { $0.contains(title) })
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [focused], timeout: 5), .completed,
                       "Expected \(title); actual: \(focusedLabels(app))", file: file, line: line)
    }
}
