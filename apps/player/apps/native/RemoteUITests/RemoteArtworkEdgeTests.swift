import UIKit
import XCTest

// Actual app pixels are required here: ImageRenderer does not model tvOS's
// native safe area. The canonical loopback fixture supplies a blue landscape.
final class RemoteArtworkEdgeTests: RemoteTestCase {
    override func setUpWithError() throws {
        try super.setUpWithError()
        let environment = ProcessInfo.processInfo.environment
        try XCTSkipUnless(environment["KINOSAIL_TV_FIXTURE_QA"] == "1"
                          && environment["KINOSAIL_TV_QUALITY_FIXTURE_MODE"] == "loaded",
                          "Requires the disposable loaded loopback artwork fixture.")
    }

    @MainActor
    func testMovieArtworkReachesViewportEdgesWhileControlsStayInset() throws {
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
        XCTAssertTrue(resume.waitForExistence(timeout: 10))

        // Wait for positive in-hero pixels and stable foreground geometry,
        // rather than accepting a screenshot taken during navigation animation.
        var previousTitle = CGRect.null
        var previousResume = CGRect.null
        var stableSamples = 0
        let loaded = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let currentTitle = title.frame
            let currentResume = resume.frame
            stableSamples = currentTitle == previousTitle && currentResume == previousResume
                ? stableSamples + 1 : 0
            previousTitle = currentTitle
            previousResume = currentResume
            guard let color = try? self.pixel(app.screenshot().image, x: 0.90, y: 0.14) else { return false }
            return stableSamples >= 2 && resume.hasFocus
                && Int(color[2]) > Int(color[1]) + 25 && Int(color[2]) > Int(color[0]) + 25
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [loaded], timeout: 10), .completed,
                       "Wait for loaded landscape, stable controls and default Resume focus")

        let screenshot = app.screenshot()
        let attachment = XCTAttachment(screenshot: screenshot)
        attachment.name = "movie viewport-edge artwork and inset controls"
        attachment.lifetime = .keepAlways
        add(attachment)
        print("ARTWORK_EDGES title=\(title.frame) resume=\(resume.frame) viewport=\(app.frame)")
        print("REMOTE movie viewport-edge artwork: \(focusedLabels(app))")

        // Foreground remains at the existing x184 inset; focus scaling has room.
        XCTAssertEqual(title.frame.minX - app.frame.minX, 184, accuracy: 2)
        XCTAssertEqual(app.buttons["My List"].frame.minX - app.frame.minX, 184, accuracy: 2)
        XCTAssertGreaterThanOrEqual(resume.frame.minX - app.frame.minX, 144)
        XCTAssertLessThanOrEqual(resume.frame.maxX, app.frame.maxX - 80)
        XCTAssertEqual(app.buttons["My List"].frame.midY,
                       app.buttons["Bookmarks"].frame.midY, accuracy: 8)
        XCTAssertTrue(resume.hasFocus)

        // The leading scrim deliberately darkens its edge. Both samples must
        // retain the known fixture's blue hue, unlike the generic sail surround.
        for y in [0.14, 0.25] {
            let leading = try pixel(screenshot.image, x: 0.001, y: y)
            XCTAssertGreaterThan(Int(leading[2]), Int(leading[1]) + 2,
                                 "Landscape must reach the leading viewport edge")
            XCTAssertGreaterThan(Int(leading[2]), Int(leading[0]) + 3)
            let trailing = try pixel(screenshot.image, x: 0.998, y: y)
            XCTAssertGreaterThan(Int(trailing[2]), Int(trailing[1]) + 25,
                                 "Landscape must reach the trailing viewport edge")
            XCTAssertGreaterThan(Int(trailing[2]), Int(trailing[0]) + 25)
        }
        let top = try pixel(screenshot.image, x: 0.90, y: 0.003)
        XCTAssertGreaterThan(Int(top[2]), Int(top[1]) + 25,
                             "Landscape must reach the upper viewport edge")
        XCTAssertGreaterThan(Int(top[2]), Int(top[0]) + 25)
    }

    private func pixel(_ image: UIImage, x: CGFloat, y: CGFloat) throws -> [UInt8] {
        let cg = try XCTUnwrap(image.cgImage)
        let sample = try XCTUnwrap(cg.cropping(to: CGRect(
            x: floor(CGFloat(cg.width) * x), y: floor(CGFloat(cg.height) * y), width: 1, height: 1)))
        var color = [UInt8](repeating: 0, count: 4)
        try color.withUnsafeMutableBytes { bytes in
            let context = try XCTUnwrap(CGContext(data: bytes.baseAddress, width: 1, height: 1,
                bitsPerComponent: 8, bytesPerRow: 4, space: CGColorSpaceCreateDeviceRGB(),
                bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
            context.draw(sample, in: CGRect(x: 0, y: 0, width: 1, height: 1))
        }
        return color
    }
}
