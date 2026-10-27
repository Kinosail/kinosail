import UIKit
import XCTest

final class ArtworkRecoveryTouchTests: XCTestCase {
    private var outageStarted = false

    override func setUpWithError() throws {
        continueAfterFailure = false
        try XCTSkipUnless(ProcessInfo.processInfo.environment["KINOSAIL_TV_FIXTURE_QA"] == "1",
                          "Requires the disposable loopback catalog fixture and a seeded simulator session.")
    }

    override func tearDown() async throws {
        if outageStarted { try await fixtureState("mode=loaded&target=library&delay=0") }
        try await super.tearDown()
    }

    @MainActor
    func testPosterReturnsAfterOutageOutlastingFirstRetries() async throws {
        XCUIDevice.shared.orientation = .portrait
        try await fixtureState("mode=failed&target=artwork&delay=60")
        outageStarted = true
        let started = Date()
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        let movies = tourTab("Movies", in: app)
        XCTAssertTrue(movies.waitForExistence(timeout: 30), app.debugDescription)
        movies.tap()
        let card = app.buttons.matching(NSPredicate(format: "label BEGINSWITH %@", "After the Rain")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 20), app.debugDescription)
        let shownSeconds = Date().timeIntervalSince(started)
        XCTAssertLessThan(shownSeconds, 25, "The first retries must start well inside the outage")
        wait(until: started.addingTimeInterval(55))
        let during = try posterCoverage(app, card: card)
        record("artwork-recovery-during-outage")
        XCTAssertLessThan(during, 0.05, "The fixture outage must keep the poster unavailable")
        var after = during
        while Date().timeIntervalSince(started) < 130 && after < 0.3 {
            wait(until: Date().addingTimeInterval(2))
            after = try posterCoverage(app, card: card)
        }
        record("artwork-recovery-after-outage")
        print("ARTWORK_RECOVERY shown=\(shownSeconds) during=\(during) after=\(after) elapsed=\(Date().timeIntervalSince(started))")
        XCTAssertGreaterThanOrEqual(after, 0.3, "The poster must return after the outage without leaving the screen")
    }

    @MainActor
    private func wait(until deadline: Date) {
        while Date() < deadline { RunLoop.current.run(until: min(deadline, Date().addingTimeInterval(0.5))) }
    }

    @MainActor
    private func record(_ name: String) {
        let attachment = XCTAttachment(screenshot: XCUIScreen.main.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }

    @MainActor
    private func posterCoverage(_ app: XCUIApplication, card: XCUIElement) throws -> Double {
        let image = try XCTUnwrap(XCUIScreen.main.screenshot().image.cgImage)
        let scale = CGFloat(image.width) / app.frame.width
        let frame = card.frame
        let poster = CGRect(x: frame.minX + frame.width * 0.15, y: frame.minY + frame.width * 0.3,
                            width: frame.width * 0.7, height: frame.width * 0.9)
        let crop = try XCTUnwrap(image.cropping(to: CGRect(x: poster.minX * scale, y: poster.minY * scale,
                                                           width: poster.width * scale, height: poster.height * scale)))
        let width = 48, height = 64
        var pixels = [UInt8](repeating: 0, count: width * height * 4)
        let context = try XCTUnwrap(CGContext(data: &pixels, width: width, height: height, bitsPerComponent: 8,
            bytesPerRow: width * 4, space: CGColorSpaceCreateDeviceRGB(),
            bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.draw(crop, in: CGRect(x: 0, y: 0, width: width, height: height))
        var art = 0
        for offset in stride(from: 0, to: pixels.count, by: 4) {
            let red = Int(pixels[offset]), green = Int(pixels[offset + 1]), blue = Int(pixels[offset + 2])
            if blue > 70 && blue > red + 25 && blue > green + 10 { art += 1 }
        }
        return Double(art) / Double(width * height)
    }

    private func fixtureState(_ query: String) async throws {
        let identity = try XCTUnwrap(ProcessInfo.processInfo.environment["KINOSAIL_TV_QUALITY_FIXTURE_ID"])
        let url = try XCTUnwrap(URL(string: "http://127.0.0.1:38359/qa/state?\(query)&identity=\(identity)"))
        let (_, response) = try await URLSession.shared.data(from: url)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
    }
}
