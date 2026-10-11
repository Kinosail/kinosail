import UIKit
import XCTest

// The loaded artwork fixture opts into twelve cast cards. Actual app pixels
// protect scroll clipping; an isolated renderer cannot model the TV safe area.
final class RemoteArtworkScrollTests: RemoteTestCase {
    override func setUpWithError() throws {
        try super.setUpWithError()
        let environment = ProcessInfo.processInfo.environment
        try XCTSkipUnless(environment["KINOSAIL_TV_FIXTURE_QA"] == "1"
                          && environment["KINOSAIL_TV_QUALITY_FIXTURE_MODE"] == "loaded"
                          && environment["KINOSAIL_TV_ARTWORK_CAST_FIXTURE"] == "1",
                          "Requires the disposable loaded artwork cast fixture.")
    }

    @MainActor
    func testCastScrollKeepsControlsClippedAndReturnsToResume() throws {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        remote.press(.down)
        remote.press(.select)
        let title = app.staticTexts["After the Rain"]
        let resume = app.buttons["Resume"]
        XCTAssertTrue(title.waitForExistence(timeout: 10))
        try waitForFocus(resume)
        let initialTitle = title.frame
        let initialResume = resume.frame
        record("artwork controls before cast scrolling", app)

        remote.press(.down)
        try waitForFocus(app.buttons["My List"])
        remote.press(.down)
        try waitForFocus(actor(1, app))
        XCTAssertEqual(app.frame, CGRect(x: 0, y: 0, width: 1920, height: 1080),
                       "This pixel assertion is scoped to the owned 1080p TV fixture")
        XCTAssertLessThan(title.frame.minY, initialTitle.minY - 20,
                          "Cast focus must exercise vertical scrolling")
        try assertInsideVerticalSafeArea(actor(1, app), app)
        record("artwork cast vertical scroll and clipped controls", app)
        // The fixture's left scrim is dark. White title/body pixels must never
        // paint into the inherited 60pt top safe area while the content scrolls.
        XCTAssertEqual(try whitePixels(app.screenshot().image,
            rect: CGRect(x: 140, y: 0, width: 800, height: 60)), 0,
            "Scrolled foreground must stay clipped to the content safe area")

        remote.press(.select)
        XCTAssertTrue(app.staticTexts["No titles available"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.staticTexts["Artwork QA Actor 01"].exists)
        record("artwork actor screen removes the detail decorative plane", app)
        remote.press(.menu)
        try waitForFocus(actor(1, app))
        record("artwork Menu restores the selected cast card", app)

        for index in 2...12 {
            remote.press(.right)
            try waitForFocus(actor(index, app))
            try assertInsideVerticalSafeArea(actor(index, app), app)
        }
        record("artwork last cast card after horizontal scrolling", app)
        for index in stride(from: 11, through: 1, by: -1) {
            remote.press(.left)
            try waitForFocus(actor(index, app))
        }
        remote.press(.up)
        try waitForFocus(app.buttons["My List"])
        remote.press(.up)
        try waitForFocus(resume)
        let restored = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            abs(title.frame.minY - initialTitle.minY) <= 2
                && abs(resume.frame.minY - initialResume.minY) <= 2
                && resume.hasFocus
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [restored], timeout: 5), .completed)
        XCTAssertEqual(title.frame.minX, 184, accuracy: 2)
        record("artwork Resume focus and inset restored after cast scrolling", app)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS %@", "After the Rain")).firstMatch
        try waitForFocus(card)
        remote.press(.menu)
        try waitForFocus(app.buttons["Movies"])
        record("artwork cast journey returns to the Movies Browse focus", app)
    }

    @MainActor private func actor(_ index: Int, _ app: XCUIApplication) -> XCUIElement {
        app.buttons.matching(NSPredicate(format: "label CONTAINS %@",
            String(format: "Artwork QA Actor %02d", index))).firstMatch
    }

    @MainActor private func waitForFocus(_ element: XCUIElement) throws {
        let focused = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            element.exists && element.hasFocus
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [focused], timeout: 5), .completed)
    }

    @MainActor private func assertInsideVerticalSafeArea(_ element: XCUIElement, _ app: XCUIApplication) throws {
        XCTAssertGreaterThanOrEqual(element.frame.minY, app.frame.minY + 58)
        XCTAssertLessThanOrEqual(element.frame.maxY, app.frame.maxY - 58)
    }

    private func whitePixels(_ image: UIImage, rect: CGRect) throws -> Int {
        let cg = try XCTUnwrap(image.cgImage)
        let sample = try XCTUnwrap(cg.cropping(to: rect))
        let context = try XCTUnwrap(CGContext(data: nil, width: sample.width, height: sample.height,
            bitsPerComponent: 8, bytesPerRow: sample.width * 4,
            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo:
                CGBitmapInfo.byteOrder32Big.rawValue | CGImageAlphaInfo.premultipliedLast.rawValue))
        context.draw(sample, in: CGRect(x: 0, y: 0, width: sample.width, height: sample.height))
        let data = try XCTUnwrap(context.data?.assumingMemoryBound(to: UInt8.self))
        return (0..<(sample.width * sample.height)).filter { pixel in
            (0..<3).allSatisfy { data[pixel * 4 + $0] > 210 }
        }.count
    }
}
