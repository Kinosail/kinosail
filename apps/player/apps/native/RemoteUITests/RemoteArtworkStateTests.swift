import UIKit
import XCTest

// Opt-in disposable-fixture journeys. Each selected case needs a fresh unique
// session seed. Prepared source is not runtime or visual acceptance evidence.
final class RemoteArtworkStateTests: RemoteTestCase {
    private var fixtureEnabled = false

    override func setUpWithError() throws {
        try super.setUpWithError()
        continueAfterFailure = false
        let env = ProcessInfo.processInfo.environment
        try XCTSkipUnless(env["KINOSAIL_TV_FIXTURE_QA"] == "1"
                          && env["KINOSAIL_TV_QUALITY_FIXTURE_MODE"] == "loaded",
                          "Requires the disposable loaded loopback fixture.")
    }

    override func tearDown() async throws {
        if fixtureEnabled { try await state("loaded") }
        try await super.tearDown()
    }

    @MainActor
    func testPendingDetailRemovesSkeletonAfterLoading() async throws {
        try requireCase("pending")
        try await state("loaded", delay: 15)
        let app = openMovie()
        let loading = app.descendants(matching: .any)["Loading your library…"]
        let pendingSeen = loading.waitForExistence(timeout: 5)
        record("artwork-detail-pending", app)
        XCTAssertTrue(pendingSeen)
        XCTAssertFalse(app.buttons["Resume"].exists)
        let pendingFrame = loading.frame
        let pendingTitleY = try firstPlaceholderY(app.screenshot().image, viewport: app.frame)
        waitForDetail(app, title: "After the Rain", timeout: 30)
        record("artwork-detail-pending-loaded", app)
        XCTAssertEqual(pendingTitleY, app.staticTexts["After the Rain"].frame.minY, accuracy: 64,
                       "The title placeholder must reserve the loaded hero's content position")
        XCTAssertFalse(loading.exists)
        XCTAssertFalse(app.buttons["Try again"].exists)
        print("ARTWORK_STATE pendingFrame=\(pendingFrame) title=\(app.staticTexts["After the Rain"].frame) resume=\(app.buttons["Resume"].frame)")
        verifyPrimaryVisible(app)
    }

    @MainActor
    func testFailedDetailRetryLoadsContent() async throws {
        try requireCase("retry")
        try await state("failed")
        let app = openMovie()
        let retry = app.buttons["Try again"]
        let retrySeen = retry.waitForExistence(timeout: 10)
        XCTAssertTrue(retrySeen)
        let focused = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in retry.hasFocus }, object: nil)
        let focusResult = await XCTWaiter.fulfillment(of: [focused], timeout: 5)
        XCTAssertEqual(focusResult, .completed)
        let heading = app.staticTexts["Couldn’t load this content"]
        let banner = app.staticTexts["Check your network and that your Server is running. Apple TV needs a connection to play."]
        if banner.exists {
            let clear = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
                heading.frame.minY > banner.frame.maxY + 12
            }, object: nil)
            let clearResult = await XCTWaiter.fulfillment(of: [clear], timeout: 5)
            record("artwork-detail-failed-settled", app)
            XCTAssertEqual(clearResult, .completed, "The connection banner must not cover the detail error heading")
            XCUIRemote.shared.press(.up)
            XCTAssertTrue(app.buttons["Reconnect"].hasFocus)
            XCUIRemote.shared.press(.down)
            XCTAssertTrue(retry.hasFocus, "Down from Reconnect must return to the visible retry action")
        }
        record("artwork-detail-failed-before-retry", app)
        XCTAssertFalse(app.descendants(matching: .any)["Loading your library…"].exists)
        XCTAssertFalse(app.buttons["Resume"].exists)
        XCTAssertTrue(retry.hasFocus, "Detail failure must expose reachable retry focus")
        try await state("loaded")
        XCUIRemote.shared.press(.select)
        waitForDetail(app, title: "After the Rain")
        record("artwork-detail-retry-loaded", app)
        XCTAssertFalse(retry.exists)
        XCTAssertFalse(app.descendants(matching: .any)["Loading your library…"].exists)
        verifyPrimaryVisible(app)
    }

    @MainActor
    func testUnavailableServerKeepsDetailRetryBelowBanner() async throws {
        try requireCase("offline")
        try await state("loaded")
        let app = openMovie(select: false)
        try await state("failed", target: "connection")
        XCUIRemote.shared.press(.select)
        let retry = app.buttons["Try again"]
        let reconnect = app.buttons["Reconnect"]
        XCTAssertTrue(retry.waitForExistence(timeout: 10))
        XCTAssertTrue(reconnect.waitForExistence(timeout: 10))
        let heading = app.staticTexts["Couldn’t load this content"]
        let banner = app.staticTexts["Check your network and that your Server is running. Apple TV needs a connection to play."]
        let clear = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            heading.frame.minY > banner.frame.maxY + 12 && retry.hasFocus
        }, object: nil)
        let result = await XCTWaiter.fulfillment(of: [clear], timeout: 5)
        record("offline detail settled retry and banner", app)
        XCTAssertEqual(result, .completed, "Server outage must leave the error heading visible and Retry focused")
        XCUIRemote.shared.press(.up)
        XCTAssertTrue(reconnect.hasFocus)
        XCUIRemote.shared.press(.down)
        XCTAssertTrue(retry.hasFocus)
        try await state("loaded")
        XCUIRemote.shared.press(.select)
        waitForDetail(app, title: "After the Rain")
        record("offline detail recovered", app)
    }

    @MainActor
    func testEmptyBackdropKeepsCinemaHeroActionsReachable() async throws {
        try requireCase("empty")
        try await state("empty")
        let detail = try await fixtureResponse("/api/v1/items/movie-004", status: 200)
        let item = try XCTUnwrap((try JSONSerialization.jsonObject(with: detail) as? [String: Any])?["item"] as? [String: Any])
        XCTAssertEqual(item["backdrop"] as? String, "")
        XCTAssertEqual(item["artwork"] as? String, "/art/movie-004")
        let app = openMovie()
        waitForDetail(app, title: "After the Rain")
        record("artwork-detail-empty-backdrop-cinema-hero", app)
        verifyPrimaryVisible(app)
        XCUIRemote.shared.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
    }

    @MainActor
    func testMissingArtworkKeepsPopulatedDetailUsable() async throws {
        try requireCase("404")
        try await state("loaded")
        let detail = try await fixtureResponse("/api/v1/items/movie-005", status: 200)
        let item = try XCTUnwrap((try JSONSerialization.jsonObject(with: detail) as? [String: Any])?["item"] as? [String: Any])
        XCTAssertEqual(item["backdrop"] as? String, "/backdrop/movie-005")
        let app = openMovie(next: true)
        waitForDetail(app, title: "Before Dawn")
        print("ARTWORK_STATE screenshotBeginUnix=\(Date().timeIntervalSince1970)")
        record("artwork-detail-backdrop-404", app)
        XCTAssertFalse(app.buttons["Try again"].exists, "Artwork404 is separate from detail failure")
        verifyPrimaryVisible(app)
        XCUIRemote.shared.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
    }

    @MainActor
    func testConfiguredAppearanceKeepsActionsReachable() async throws {
        try requireCase("appearance")
        let variant = ProcessInfo.processInfo.environment["KINOSAIL_TV_ARTWORK_APPEARANCE"] ?? ""
        try XCTSkipUnless(["light", "dark", "large-text", "increased-contrast", "reduce-transparency"].contains(variant),
                          "Requires separately verified real simulator settings; this label changes no app environment.")
        try await state("loaded")
        let app = openMovie()
        waitForDetail(app, title: "After the Rain")
        record("artwork-detail-appearance-\(variant)-primary", app)
        verifyPrimaryVisible(app)
        XCUIRemote.shared.press(.down)
        record("artwork-detail-appearance-\(variant)-first-down", app)
        let bookmarks = app.buttons["Bookmarks"]
        let myList = app.buttons["My List"]
        waitForFocus(myList, in: app)
        if abs(bookmarks.frame.midY - myList.frame.midY) <= 8 {
            XCUIRemote.shared.press(.right)
            waitForFocus(bookmarks, in: app)
            XCUIRemote.shared.press(.left)
            waitForFocus(myList, in: app)
        } else {
            XCTAssertLessThan(myList.frame.minY, bookmarks.frame.minY)
            XCUIRemote.shared.press(.down)
            waitForFocus(bookmarks, in: app)
            XCUIRemote.shared.press(.up)
            waitForFocus(myList, in: app)
        }
        record("artwork-detail-appearance-\(variant)-secondary-access", app)
        XCUIRemote.shared.press(.up)
        waitForFocus(app.buttons["Resume"], in: app)
        record("artwork-detail-appearance-\(variant)-return", app)
        verifyPrimaryVisible(app)
    }

    // The placeholder intentionally hides its decorative children from accessibility.
    // Inspect actual pixels at the shared text inset, without a production test hook.
    private func firstPlaceholderY(_ image: UIImage, viewport: CGRect) throws -> CGFloat {
        let cg = try XCTUnwrap(image.cgImage)
        let scale = CGFloat(cg.width) / viewport.width
        let column = try XCTUnwrap(cg.cropping(to: CGRect(x: 204 * scale, y: 0, width: 1, height: CGFloat(cg.height))))
        let context = try XCTUnwrap(CGContext(data: nil, width: 1, height: column.height,
            bitsPerComponent: 8, bytesPerRow: 4, space: CGColorSpaceCreateDeviceRGB(),
            bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.draw(column, in: CGRect(x: 0, y: 0, width: 1, height: column.height))
        let pixels = try XCTUnwrap(context.data?.assumingMemoryBound(to: UInt8.self))
        let rows = Int(100 * scale)..<Int(600 * scale)
        let y = try XCTUnwrap(rows.first { row in
            let offset = row * 4
            return pixels[offset] >= 28 && pixels[offset + 1] >= 34
                && pixels[offset + 1] > pixels[offset + 2]
        }, "The pending screen must contain a visible title placeholder")
        return CGFloat(y) / scale
    }

    @MainActor
    func openMovie(next: Bool = false, select: Bool = true) -> XCUIApplication {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        if next { print("ARTWORK_STATE appLaunchCompletedUnix=\(Date().timeIntervalSince1970)") }
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        remote.press(.down)
        if next {
            remote.press(.right)
            remote.press(.right)
        }
        XCTAssertTrue(focusedLabels(app).joined().contains(next ? "Before Dawn" : "After the Rain"))
        if select { remote.press(.select) }
        return app
    }

    @MainActor
    func waitForDetail(_ app: XCUIApplication, title: String, timeout: TimeInterval = 10) {
        let titleSeen = app.staticTexts[title].waitForExistence(timeout: timeout)
        if !titleSeen { record("artwork-detail-title-wait-failure", app) }
        XCTAssertTrue(titleSeen)
        let resumeSeen = app.buttons["Resume"].waitForExistence(timeout: timeout)
        if !resumeSeen { record("artwork-detail-resume-wait-failure", app) }
        XCTAssertTrue(resumeSeen)
        waitForFocus(app.buttons["Resume"], in: app)
    }

    @MainActor
    func waitForFocus(_ element: XCUIElement, in app: XCUIApplication) {
        var previous = CGRect.null
        var stable = 0
        let focused = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let current = element.frame
            stable = current == previous ? stable + 1 : 0
            previous = current
            return stable >= 2 && element.hasFocus && element.isHittable
        }, object: nil)
        let result = XCTWaiter.wait(for: [focused], timeout: 5)
        if result != .completed {
            record("artwork-detail-focus-wait-failure", app)
        }
        XCTAssertEqual(result, .completed)
    }

    @MainActor
    func verifyPrimaryVisible(_ app: XCUIApplication) {
        let resume = app.buttons["Resume"]
        XCTAssertTrue(resume.hasFocus)
        XCTAssertTrue(resume.isHittable)
        XCTAssertGreaterThanOrEqual(resume.frame.minX, app.frame.minX)
        XCTAssertLessThanOrEqual(resume.frame.maxX, app.frame.maxX)
        XCTAssertGreaterThanOrEqual(resume.frame.minY, app.frame.minY)
        XCTAssertLessThanOrEqual(resume.frame.maxY, app.frame.maxY)
        print("ARTWORK_STATE resume=\(resume.frame) viewport=\(app.frame) focused=\(focusedLabels(app))")
    }

    func requireCase(_ name: String) throws {
        try XCTSkipUnless(ProcessInfo.processInfo.environment["KINOSAIL_TV_ARTWORK_STATE_CASE"] == name,
                          "Select one case after its own unique disposable session seed.")
        fixtureEnabled = true
    }

    func state(_ mode: String, delay: Int = 0, target: String = "details") async throws {
        let identity = try XCTUnwrap(ProcessInfo.processInfo.environment["KINOSAIL_TV_QUALITY_FIXTURE_ID"])
        XCTAssertNotNil(identity.range(of: "^tv-polish-qa-[a-z0-9-]{1,64}$", options: .regularExpression),
                        "A unique freshly seeded identity is required; the bare fallback is rejected")
        let url = URL(string: "http://127.0.0.1:38359/qa/state?mode=\(mode)&delay=\(delay)&target=\(target)&identity=\(identity)")!
        let (_, response) = try await URLSession.shared.data(from: url)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
    }

    private func fixtureResponse(_ path: String, status: Int) async throws -> Data {
        var request = URLRequest(url: URL(string: "http://127.0.0.1:38359\(path)")!)
        request.setValue("Bearer tv-polish-fixture-token", forHTTPHeaderField: "Authorization")
        let (data, response) = try await URLSession.shared.data(for: request)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, status)
        return data
    }

}
