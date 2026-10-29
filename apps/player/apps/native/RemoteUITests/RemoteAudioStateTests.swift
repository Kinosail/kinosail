import UIKit
import XCTest

final class RemoteAudioStateTests: RemoteTestCase {
    private var fixtureEnabled = false
    override func setUpWithError() throws {
        try super.setUpWithError()
        continueAfterFailure = false
        let environment = ProcessInfo.processInfo.environment
        try XCTSkipUnless(environment["KINOSAIL_TV_FIXTURE_QA"] == "1"
                          && environment["KINOSAIL_TV_AUDIO_STATE_CASE"] == "1")
        fixtureEnabled = true
    }

    override func tearDown() async throws {
        if fixtureEnabled { try await state("loaded") }
        try await super.tearDown()
    }

    @MainActor func testPendingAudioRemovesSkeletonAndFocusesTransport() async throws {
        try await state("loaded", delay: 15)
        let app = openTrack()
        let loading = app.descendants(matching: .any).matching(identifier: "Opening audio…").firstMatch
        XCTAssertTrue(loading.waitForExistence(timeout: 5))
        record("audio pending", app)
        let pendingImage = app.screenshot().image
        XCTAssertFalse(app.buttons["More"].exists)
        XCTAssertFalse(app.buttons["Playback options"].exists)
        XCTAssertFalse(app.buttons["Options"].exists)
        try await state("loaded")
        XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 25))
        XCTAssertTrue(app.buttons["Pause"].hasFocus)
        XCUIRemote.shared.press(.select)
        XCTAssertTrue(app.buttons["Play"].waitForExistence(timeout: 5))
        XCTAssertFalse(loading.exists)
        record("audio loaded after pending", app)
        let play = app.buttons["Play"]
        let notice = app.staticTexts.matching(NSPredicate(format: "label BEGINSWITH 'Progress is saved on this device.'")).firstMatch
        let noticeHeight = notice.exists ? notice.frame.height + 28 : 0
        let pendingY = try pendingTransportCenter(pendingImage, x: play.frame.midX, viewport: app.frame)
        print("AUDIO_GEOMETRY pendingTransportY=\(pendingY) loadedTransportY=\(play.frame.midY) conditionalNoticeHeight=\(noticeHeight)")
        XCTAssertEqual(pendingY, play.frame.midY - noticeHeight, accuracy: 20,
                       "The pending transport must reserve the loaded controls' geometry, excluding a new failure notice")
    }

    @MainActor func testFailedAudioExposesRetryAndRecoversTransport() async throws {
        try await state("failed")
        let app = openTrack()
        let retry = app.buttons["Try again"]
        XCTAssertTrue(retry.waitForExistence(timeout: 15))
        let focused = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in retry.hasFocus }, object: nil)
        let focusResult = await XCTWaiter.fulfillment(of: [focused], timeout: 5)
        record("audio failure settled focus", app)
        XCTAssertEqual(focusResult, .completed)
        XCTAssertFalse(app.descendants(matching: .any).matching(identifier: "Opening audio…").firstMatch.exists)
        record("audio failed with reachable retry", app)
        try await state("loaded")
        XCUIRemote.shared.press(.select)
        XCTAssertTrue(app.buttons["Pause"].waitForExistence(timeout: 20))
        XCTAssertTrue(app.buttons["Pause"].hasFocus)
        XCUIRemote.shared.press(.select)
        XCTAssertTrue(app.buttons["Play"].waitForExistence(timeout: 5))
        XCTAssertFalse(retry.exists)
        record("audio recovered after retry", app)
        XCUIRemote.shared.press(.menu)
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 10))
    }

    @MainActor private func openTrack() -> XCUIApplication {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        launchAtSearch(app)
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        remote.press(.right)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Music"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Albums"].waitForExistence(timeout: 10))
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 10))
        remote.press(.select)
        return app
    }

    private func pendingTransportCenter(_ image: UIImage, x: CGFloat, viewport: CGRect) throws -> CGFloat {
        let cg = try XCTUnwrap(image.cgImage)
        let scale = CGFloat(cg.width) / viewport.width
        let column = try XCTUnwrap(cg.cropping(to: CGRect(x: x * scale, y: 0, width: 1, height: CGFloat(cg.height))))
        let context = try XCTUnwrap(CGContext(data: nil, width: 1, height: column.height,
            bitsPerComponent: 8, bytesPerRow: 4, space: CGColorSpaceCreateDeviceRGB(),
            bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.draw(column, in: CGRect(x: 0, y: 0, width: 1, height: column.height))
        let pixels = try XCTUnwrap(context.data?.assumingMemoryBound(to: UInt8.self))
        var start: Int?
        for row in Int(300 * scale)..<Int(750 * scale) {
            let color = [21, 25, 20].enumerated().allSatisfy { abs(Int(pixels[row * 4 + $0.offset]) - $0.element) <= 2 }
            if color { if start == nil { start = row } }
            else if let first = start {
                if CGFloat(row - first) > 80 * scale { return CGFloat(first + row) / (2 * scale) }
                start = nil
            }
        }
        throw NSError(domain: "Pending transport circle was not rendered", code: 1)
    }

    private func state(_ mode: String, delay: Int = 0) async throws {
        let identity = try XCTUnwrap(ProcessInfo.processInfo.environment["KINOSAIL_TV_QUALITY_FIXTURE_ID"])
        XCTAssertNotNil(identity.range(of: "^tv-polish-qa-[a-z0-9-]{1,64}$", options: .regularExpression))
        let url = try XCTUnwrap(URL(string: "http://127.0.0.1:38359/qa/state?mode=\(mode)&delay=\(delay)&target=playback&identity=\(identity)"))
        let (_, response) = try await URLSession.shared.data(from: url)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
    }
}
