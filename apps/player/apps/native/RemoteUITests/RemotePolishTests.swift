import UIKit
import XCTest

// Uses the populated loopback QA session. Keep screenshot attachments in the result bundle.
final class RemotePolishTests: RemoteTestCase {
    @MainActor
    func testPlayPauseStartsTheFocusedHomeTitle() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        XCUIRemote.shared.press(.down)
        XCUIRemote.shared.press(.playPause)
        XCTAssertTrue(app.cells["Playback options"].waitForExistence(timeout: 20))
        record("home quick play", app)
        XCUIRemote.shared.press(.menu)
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 10))
        record("home after quick play", app)
    }

    @MainActor
    func testFocusedContinueWatchingCardsKeepTheirNeighborsClear() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        record("home search", app)
        remote.press(.down)
        let titles = ["After the Rain", "An Extraordinary Journey", "Before Dawn", "Moonrise", "Night Sky"]
        var comparisons = 0
        for index in 0..<4 {
            let focused = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
            XCTAssertTrue(focused.exists)
            XCTAssertTrue(focused.label.contains(titles[index]))
            let next = app.buttons.matching(NSPredicate(format: "label CONTAINS %@", titles[index + 1])).firstMatch
            XCTAssertTrue(next.exists)
            if next.isHittable && abs(next.frame.midY - focused.frame.midY) < 60 && next.frame.minX > focused.frame.minX {
                XCTAssertLessThanOrEqual(focused.frame.maxX + 8, next.frame.minX,
                                         "The focused card must leave breathing room before the next card")
                comparisons += 1
            }
            record("watching focus \(focused.label)", app)
            remote.press(.right)
        }
        remote.press(.up)
        XCTAssertTrue(app.buttons["Search"].hasFocus)
        XCTAssertGreaterThanOrEqual(comparisons, 2)
    }

    @MainActor
    func testAudioOptionsKeepUnfocusedLabelsLegible() {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        remote.press(.right)
        remote.press(.right)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Albums"].waitForExistence(timeout: 10))
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 10))
        remote.press(.select)
        XCTAssertTrue(app.buttons["Back 15 seconds"].waitForExistence(timeout: 20))
        for _ in 0..<5 {
            let focused = app.descendants(matching: .any).matching(NSPredicate(format: "hasFocus == true")).firstMatch
            XCTAssertTrue(focused.waitForExistence(timeout: 5), "Focus should settle after the player transition")
            if focused.label == "Shuffle off" || focused.label == "Repeat: off" { break }
            remote.press(.down)
        }
        let sleep = app.descendants(matching: .any).matching(NSPredicate(format: "label == 'Sleep timer'")).firstMatch
        XCTAssertTrue(sleep.exists)
        XCTAssertFalse(sleep.hasFocus)
        let frame = sleep.frame.insetBy(dx: 25, dy: 15)
        let image = app.screenshot().image
        let viewport = app.frame
        var lightSamples = 0
        for x in stride(from: frame.minX, through: frame.maxX, by: 3) {
            for y in stride(from: frame.minY, through: frame.maxY, by: 3) {
                let color = pixel(image, x: x / viewport.width, y: y / viewport.height)
                if color[0] > 220 && color[1] > 220 && color[2] > 220 { lightSamples += 1 }
            }
        }
        XCTAssertGreaterThan(lightSamples, 20, "An unfocused option must keep a readable light label")
        record("audio option focus and neighboring labels", app)
    }

    @MainActor
    func testMovieAndShowHeroArtworkActuallyRenders() {
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
        XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10))
        assertBlueFixtureArtwork(app, name: "movie hero artwork")
        XCTAssertEqual(app.buttons["My List"].frame.midY, app.buttons["Bookmarks"].frame.midY, accuracy: 8,
                       "Secondary movie actions should share a compact row")
        remote.press(.menu)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Movies"].waitForExistence(timeout: 10))
        remote.press(.right)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        remote.press(.down)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Episodes"].waitForExistence(timeout: 10))
        assertBlueFixtureArtwork(app, name: "show hero artwork")
    }

    @MainActor
    private func assertBlueFixtureArtwork(_ app: XCUIApplication, name: String) {
        let predicate = NSPredicate { _, _ in
            let image = app.screenshot().image
            // Sample the artwork away from controls; the moon and crop vary with hero height.
            return [0.65, 0.9].contains { x in
                [0.14, 0.25, 0.38].contains { y in
                    let color = self.pixel(image, x: x, y: y)
                    return Int(color[2]) > Int(color[1]) + 25 && Int(color[2]) > Int(color[0]) + 25
                }
            }
        }
        let wait = XCTNSPredicateExpectation(predicate: predicate, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [wait], timeout: 10), .completed,
                       "The fixture's landscape must render inside the hero")
        record(name, app)
    }

    private func pixel(_ image: UIImage, x: CGFloat, y: CGFloat) -> [UInt8] {
        guard let cg = image.cgImage,
              let sample = cg.cropping(to: CGRect(x: CGFloat(cg.width) * x, y: CGFloat(cg.height) * y, width: 1, height: 1))
        else { return [0, 0, 0, 0] }
        var color = [UInt8](repeating: 0, count: 4)
        color.withUnsafeMutableBytes { bytes in
            let context = CGContext(data: bytes.baseAddress, width: 1, height: 1, bitsPerComponent: 8,
                                    bytesPerRow: 4, space: CGColorSpaceCreateDeviceRGB(),
                                    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
            context.draw(sample, in: CGRect(x: 0, y: 0, width: 1, height: 1))
        }
        return color
    }
}
