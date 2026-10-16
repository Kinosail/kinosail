import UIKit
import XCTest

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
        launchAtSearch(app)
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
        try waitForLoadedHero(title, resume, app)
        let initialTitle = title.frame
        let initialResume = resume.frame
        record("artwork controls before cast scrolling", app)

        remote.press(.down)
        record("artwork first Down focus before My List validation", app)
        try waitForFocus(app.buttons["My List"], diagnosticApp: app)
        remote.press(.down)
        try waitForFocus(actor(1, app), diagnosticApp: app)
        let scrolled = app.screenshot()
        let attachment = XCTAttachment(screenshot: scrolled)
        attachment.name = "artwork cast vertical scroll and clipped controls"
        attachment.lifetime = .keepAlways
        add(attachment)
        print("ARTWORK_SCROLL title=\(title.frame) cast=\(actor(1, app).frame) viewport=\(app.frame)")
        let firstCastFrame = actor(1, app).frame
        XCTAssertEqual(app.frame, CGRect(x: 0, y: 0, width: 1920, height: 1080),
                       "This pixel assertion is scoped to the owned 1080p TV fixture")
        XCTAssertLessThan(title.frame.minY, initialTitle.minY - 20,
                          "Cast focus must exercise vertical scrolling")
        try assertInsideVerticalSafeArea(actor(1, app), app)
        XCTAssertEqual(try foregroundPixels(scrolled.image,
            rect: CGRect(x: 140, y: 0, width: 800, height: 60)), 0,
            "Scrolled foreground must stay clipped to the content safe area")

        remote.press(.select)
        XCTAssertTrue(app.staticTexts["No titles available"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.staticTexts["Artwork QA Actor 01"].exists)
        record("artwork actor screen removes the detail decorative plane", app)
        remote.press(.menu)
        try waitForFocus(actor(1, app), diagnosticApp: app)
        record("artwork Menu restores the selected cast card", app)

        for index in 2...12 {
            remote.press(.right)
            try waitForFocus(actor(index, app))
            record("artwork cast card \(index) after horizontal scrolling", app)
            try assertInsideVerticalSafeArea(actor(index, app), app)
        }
        let last = actor(12, app)
        XCTAssertGreaterThanOrEqual(last.frame.minX, app.frame.minX + 78)
        XCTAssertLessThanOrEqual(last.frame.maxX, app.frame.maxX - 78)
        let first = actor(1, app)
        XCTAssertTrue(!first.exists || first.frame.minX < firstCastFrame.minX - 230,
                      "Reaching the last visible cast card must exercise horizontal scrolling")
        for index in stride(from: 11, through: 1, by: -1) {
            remote.press(.left)
            try waitForFocus(actor(index, app))
        }
        remote.press(.up)
        try waitForFocus(app.buttons["My List"])
        remote.press(.up)
        try waitForFocus(resume)
        record("artwork return to Resume before loaded validation", app)
        try waitForLoadedHero(title, resume, app)
        record("artwork Resume focus and inset restored after cast scrolling", app)
        let restored = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            abs(title.frame.minY - initialTitle.minY) <= 2
                && abs(resume.frame.minY - initialResume.minY) <= 2
                && resume.hasFocus
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [restored], timeout: 5), .completed)
        XCTAssertEqual(title.frame.minX, 184, accuracy: 2)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS %@", "After the Rain")).firstMatch
        try waitForFocus(card)
        remote.press(.menu)
        try waitForFocus(app.buttons["Movies"])
        record("artwork cast journey returns to the Movies Browse focus", app)
    }

    @MainActor
    func testCastReturnRestoresInitialHeroFrames() throws {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
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
        try waitForLoadedHero(title, resume, app)
        let initialTitle = title.frame
        let initialResume = resume.frame
        record("focused artwork restoration initial hero", app)

        remote.press(.down)
        try waitForFocus(app.buttons["My List"])
        remote.press(.down)
        try waitForFocus(actor(1, app), diagnosticApp: app)
        let scrolled = app.screenshot()
        let attachment = XCTAttachment(screenshot: scrolled)
        attachment.name = "focused artwork restoration scrolled Cast"
        attachment.lifetime = .keepAlways
        add(attachment)
        XCTAssertEqual(app.frame, CGRect(x: 0, y: 0, width: 1920, height: 1080))
        XCTAssertLessThan(title.frame.minY, initialTitle.minY - 20,
                          "The restoration check must exercise vertical scrolling")
        try assertInsideVerticalSafeArea(actor(1, app), app)
        XCTAssertEqual(try foregroundPixels(scrolled.image,
            rect: CGRect(x: 140, y: 0, width: 800, height: 60)), 0)

        remote.press(.up)
        try waitForFocus(app.buttons["My List"])
        remote.press(.up)
        try waitForFocus(resume)
        try waitForLoadedHero(title, resume, app)
        let restored = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            abs(title.frame.minY - initialTitle.minY) <= 2
                && abs(resume.frame.minY - initialResume.minY) <= 2
                && resume.hasFocus
        }, object: nil)
        let result = XCTWaiter.wait(for: [restored], timeout: 5)
        let observedTitle = title.frame
        let observedResume = resume.frame
        print("ARTWORK_RESTORE expectedTitle=\(initialTitle) expectedResume=\(initialResume) observedTitle=\(observedTitle) observedResume=\(observedResume) resumeFocused=\(resume.hasFocus) completed=\(result == .completed)")
        record("focused artwork restoration returned hero before assertion", app)
        XCTAssertEqual(result, .completed)
        XCTAssertEqual(title.frame.minX, 184, accuracy: 2)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS %@", "After the Rain")).firstMatch
        try waitForFocus(card)
        remote.press(.menu)
        try waitForFocus(app.buttons["Movies"])
        record("focused artwork restoration returns to Movies Browse focus", app)
    }

    @MainActor private func actor(_ index: Int, _ app: XCUIApplication) -> XCUIElement {
        app.buttons.matching(NSPredicate(format: "label CONTAINS %@",
            String(format: "Artwork QA Actor %02d", index))).firstMatch
    }

    @MainActor private func waitForLoadedHero(_ title: XCUIElement, _ resume: XCUIElement,
                                             _ app: XCUIApplication) throws {
        var previousTitle = CGRect.null
        var previousResume = CGRect.null
        var stableSamples = 0
        let loaded = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            guard resume.exists else { return false }
            let currentTitle = title.frame
            let currentResume = resume.frame
            stableSamples = currentTitle == previousTitle && currentResume == previousResume
                ? stableSamples + 1 : 0
            previousTitle = currentTitle
            previousResume = currentResume
            guard let color = try? self.color(app.screenshot().image, x: 1728, y: 151) else { return false }
            return stableSamples >= 2 && resume.hasFocus
                && Int(color[2]) > Int(color[1]) + 25 && Int(color[2]) > Int(color[0]) + 25
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [loaded], timeout: 10), .completed,
                       "Wait for loaded landscape and stable controls before scroll measurements")
    }

    @MainActor private func assertInsideVerticalSafeArea(_ element: XCUIElement, _ app: XCUIApplication) throws {
        XCTAssertGreaterThanOrEqual(element.frame.minY, app.frame.minY + 58)
        XCTAssertLessThanOrEqual(element.frame.maxY, app.frame.maxY - 58)
    }

    private func foregroundPixels(_ image: UIImage, rect: CGRect) throws -> Int {
        let cg = try XCTUnwrap(image.cgImage)
        let sample = try XCTUnwrap(cg.cropping(to: rect))
        let context = try XCTUnwrap(CGContext(data: nil, width: sample.width, height: sample.height,
            bitsPerComponent: 8, bytesPerRow: sample.width * 4,
            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo:
                CGBitmapInfo.byteOrder32Big.rawValue | CGImageAlphaInfo.premultipliedLast.rawValue))
        context.draw(sample, in: CGRect(x: 0, y: 0, width: CGFloat(sample.width), height: CGFloat(sample.height)))
        let data = try XCTUnwrap(context.data?.assumingMemoryBound(to: UInt8.self))
        return (0..<(sample.width * sample.height)).filter { pixel in
            (0..<3).allSatisfy { data[pixel * 4 + $0] > 120 }
        }.count
    }

    private func color(_ image: UIImage, x: Int, y: Int) throws -> [UInt8] {
        let cg = try XCTUnwrap(image.cgImage)
        let sample = try XCTUnwrap(cg.cropping(to: CGRect(x: CGFloat(x), y: CGFloat(y), width: 1, height: 1)))
        let context = try XCTUnwrap(CGContext(data: nil, width: 1, height: 1,
            bitsPerComponent: 8, bytesPerRow: 4, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo:
                CGBitmapInfo.byteOrder32Big.rawValue | CGImageAlphaInfo.premultipliedLast.rawValue))
        context.draw(sample, in: CGRect(x: 0, y: 0, width: 1, height: 1))
        let data = try XCTUnwrap(context.data?.assumingMemoryBound(to: UInt8.self))
        return [data[0], data[1], data[2]]
    }
}
