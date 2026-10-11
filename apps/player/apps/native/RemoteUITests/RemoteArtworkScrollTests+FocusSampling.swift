import UIKit
import XCTest

extension RemoteArtworkScrollTests {
    // Run these controls explicitly before admitting a sampling-helper repair.
    // The two negative controls intentionally fail the existing helper assertion.
    @MainActor
    func testFocusSamplingControlRequiresTwoSecondsBeforeCompleting() throws {
        let app = try launchFocusSamplingControl()
        let movies = app.buttons["Movies"]
        let started = ProcessInfo.processInfo.systemUptime
        try waitForFocus(movies, diagnosticApp: app)
        let elapsed = ProcessInfo.processInfo.systemUptime - started
        XCTAssertGreaterThanOrEqual(elapsed, 2,
            "Immediate sampling must retain the two-second settling interval")
        XCTAssertTrue(movies.hasFocus)
        record("focus sampling control completed with Movies focused", app)
    }

    @MainActor
    func testFocusSamplingControlRejectsStableUnfocusedTarget() throws {
        let app = try launchFocusSamplingControl()
        let search = app.buttons["Search"]
        XCTAssertTrue(search.exists)
        XCTAssertFalse(search.hasFocus)
        record("unfocused Search control before focus wait", app)
        continueAfterFailure = true
        try waitForFocus(search)
        continueAfterFailure = false
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        XCTAssertFalse(search.hasFocus)
        record("unfocused Search control rejected with Movies still focused", app)
    }

    @MainActor
    func testFocusSamplingControlRejectsMissingTarget() throws {
        let app = try launchFocusSamplingControl()
        let missing = app.buttons["artwork.qa.focus-control-missing-target"]
        XCTAssertFalse(missing.exists)
        record("missing target control before focus wait", app)
        continueAfterFailure = true
        try waitForFocus(missing)
        continueAfterFailure = false
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        XCTAssertFalse(missing.exists)
        record("missing target control rejected with Movies still focused", app)
    }

    @MainActor private func launchFocusSamplingControl() throws -> XCUIApplication {
        try XCTSkipUnless(ProcessInfo.processInfo.environment["KINOSAIL_TV_FOCUS_SAMPLING_CONTROLS"] == "1",
            "Requires an explicitly selected artwork focus-sampling control")
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        XCUIRemote.shared.press(.down)
        XCUIRemote.shared.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        return app
    }

    @MainActor func waitForFocus(_ element: XCUIElement,
                                         diagnosticApp: XCUIApplication? = nil) throws {
        let deadline = ProcessInfo.processInfo.systemUptime + 5
        var stableSince = ProcessInfo.processInfo.systemUptime
        var previous = CGRect.null
        var stableSamples = 0
        let predicate = NSPredicate { _, _ in
            guard ProcessInfo.processInfo.systemUptime < deadline,
                  element.exists else { return false }
            let frame = element.frame
            if frame != previous {
                stableSince = ProcessInfo.processInfo.systemUptime
            }
            stableSamples = frame == previous ? stableSamples + 1 : 0
            previous = frame
            let hasFocus = element.hasFocus
            if diagnosticApp != nil {
                print("ARTWORK_FOCUS_WAIT label=\(element.label) focused=\(hasFocus) frame=\(frame) stable=\(stableSamples)")
            }
            return hasFocus && stableSamples >= 2
                && ProcessInfo.processInfo.systemUptime - stableSince >= 2
                && ProcessInfo.processInfo.systemUptime < deadline
        }
        // Capture the first sample now; keep three samples over at least two seconds.
        _ = predicate.evaluate(with: nil)
        let focused = XCTNSPredicateExpectation(predicate: predicate, object: nil)
        let remaining = max(0, deadline - ProcessInfo.processInfo.systemUptime)
        let result: XCTWaiter.Result = remaining > 0
            ? XCTWaiter.wait(for: [focused], timeout: remaining) : .timedOut
        if result != .completed, let diagnosticApp {
            record("artwork My List focus wait failure before assertion", diagnosticApp)
        }
        XCTAssertEqual(result, .completed)
    }

}
