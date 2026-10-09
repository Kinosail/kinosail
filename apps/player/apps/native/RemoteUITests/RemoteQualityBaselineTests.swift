import XCTest

final class RemoteQualityBaselineTests: RemoteTestCase {
    override func setUpWithError() throws {
        try super.setUpWithError()
        try XCTSkipUnless(ProcessInfo.processInfo.environment["KINOSAIL_TV_FIXTURE_QA"] == "1",
                          "Requires the disposable loopback catalog and QA Viewer Profile.")
    }
    @MainActor func testDeepHomeBackReachesSearchAndDetailBackRestoresCard() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        XCTAssertTrue(app.buttons["Search"].hasFocus)
        var movements = 0
        for _ in 0..<16 {
            let card = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
            if app.staticTexts["Nature"].isHittable && card.exists,
               card.frame.minY > app.staticTexts["Nature"].frame.maxY { break }
            remote.press(.down)
            movements += 1
        }
        XCTAssertTrue(app.staticTexts["Nature"].isHittable)
        let card = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
        XCTAssertGreaterThan(card.frame.minY, app.staticTexts["Nature"].frame.maxY,
                             "The selected card must belong to the final genre shelf")
        let originalFrame = settledFrame(card)
        let original = focusedLabels(app).first ?? ""
        XCTAssertFalse(original.isEmpty)
        print("QUALITY movements Search-to-final-genre: \(movements) Down")
        record("quality-deep-home-origin", app)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10))
        record("quality-detail-before-back", app)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Nature"].waitForExistence(timeout: 10))
        XCTAssertEqual(focusedLabels(app).first, original, "Detail Back must restore its originating card before any shortcut")
        XCTAssertEqual(settledFrame(card).midY, originalFrame.midY, accuracy: 8,
                       "A duplicate title in a different shelf is not the originating card")
        record("quality-detail-back-restored-card", app)
        remote.press(.menu)
        record("quality-one-back-from-deep-home", app)
        XCTAssertEqual(app.state, .runningForeground, "Home Back should first reach global navigation")
        XCTAssertTrue(app.buttons["Search"].hasFocus, "One Back from a Home row should reach Search")
        remote.press(.down)
        let returned = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
        XCTAssertTrue(returned.waitForExistence(timeout: 5))
        record("quality-search-down-after-shortcut", app)
        XCTAssertTrue(returned.label.contains("An Extraordinary Journey"),
                      "Down must reach the nearest visible Mystery row below Search")
        XCTAssertLessThan(settledFrame(returned).midY, app.staticTexts["Nature"].frame.minY)
        XCTAssertTrue(returned.isHittable)
        remote.press(.down)
        XCTAssertEqual(focusedLabels(app).first, original,
                       "The next Down must reach the original Nature card")
        record("quality-search-two-downs-original-shelf", app)
        print("QUALITY movements final-genre-to-Search: 1 Back; Search-to-origin: 2 Down")
    }

    @MainActor func testContinueWatchingDirectPlayReturnsBeforeHomeShortcutAndRootExit() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        remote.press(.down)
        let origin = focusedLabels(app).first ?? ""
        XCTAssertTrue(origin.contains("After the Rain"))
        for (input, select) in [("Select", true), ("Play-Pause", false)] {
            remote.press(select ? .select : .playPause)
            XCTAssertTrue(app.cells["Playback options"].waitForExistence(timeout: 20))
            record("quality-continue-watching-\(input)-player", app)
            remote.press(.menu)
            XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 10))
            XCTAssertEqual(focusedLabels(app).first, origin,
                           "Player Back must restore the card without corrective movement")
            record("quality-continue-watching-\(input)-restored", app)
        }
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Search"].hasFocus)
        remote.press(.menu)
        let exits = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            app.state != .runningForeground
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [exits], timeout: 5), .completed,
                       "Back from global navigation must preserve the system exit")
        print("QUALITY root Back app-state: \(app.state.rawValue)")
    }

    @MainActor func testOneHundredDetailReturnsPreserveOriginatingShelf() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        remote.press(.down)
        let card = app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch
        XCTAssertTrue(card.exists)
        let origin = card.label
        let originalFrame = settledFrame(card)
        for trial in 1...100 {
            remote.press(.select)
            XCTAssertTrue(app.buttons["Bookmarks"].waitForExistence(timeout: 10))
            remote.press(.menu)
            XCTAssertTrue(app.staticTexts["Recently added movies"].waitForExistence(timeout: 10))
            let label = focusedLabels(app).first
            let returnedFrame = settledFrame(card)
            if label != origin || abs(returnedFrame.midY - originalFrame.midY) > 8 {
                record("quality-detail-restoration-failure-\(trial)", app)
                print("QUALITY restoration geometry trial \(trial): original=\(originalFrame), returned=\(returnedFrame)")
                print(app.debugDescription)
            }
            XCTAssertEqual(label, origin, "Detail return trial \(trial)")
            XCTAssertEqual(returnedFrame.midY, originalFrame.midY, accuracy: 8,
                           "Shelf restoration trial \(trial)")
            print("QUALITY detail restoration trial \(trial): passed")
            if trial % 20 == 0 { record("quality-detail-restoration-\(trial)", app) }
        }
        remote.press(.right, forDuration: 1)
        XCTAssertNotEqual(focusedLabels(app).first, origin)
        remote.press(.left, forDuration: 1)
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "hasFocus == true")).firstMatch.isHittable)
        remote.press(.menu)
        XCTAssertTrue(app.buttons["Search"].hasFocus)
        record("quality-held-reversal-and-home-shortcut", app)
    }

    @MainActor private func settledFrame(_ element: XCUIElement) -> CGRect {
        var previous = element.frame
        var stableSamples = 0
        let stable = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let current = element.frame
            stableSamples = current == previous ? stableSamples + 1 : 0
            previous = current
            return stableSamples >= 2
        }, object: nil)
        XCTAssertEqual(XCTWaiter.wait(for: [stable], timeout: 5), .completed,
                       "Focus geometry must settle before measuring scroll restoration")
        return element.frame
    }

    @MainActor func testSeasonChangeKeepsSeasonFocusAndDownActivatesItsEpisode() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.right)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        remote.press(.down)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Episodes"].waitForExistence(timeout: 10))
        remote.press(.down)
        XCTAssertTrue(app.buttons["show.season-1"].hasFocus)
        remote.press(.right)
        XCTAssertTrue(app.buttons["show.season-2"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.buttons["show.season-2"].hasFocus, "Changing season must keep focus on the selected season")
        remote.press(.down)
        let episode = focusedLabels(app).joined(separator: ", ")
        XCTAssertTrue(episode.contains("A Quiet Evening"), "Down must reach the selected season's first episode")
        record("quality-season-two-first-episode", app)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["A Quiet Evening"].waitForExistence(timeout: 10))
        XCTAssertTrue(app.buttons["Bookmarks"].exists)
        record("quality-season-two-episode-detail", app)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Episodes"].waitForExistence(timeout: 10))
        XCTAssertEqual(focusedLabels(app).joined(separator: ", "), episode)
        record("quality-season-two-restored-episode", app)
    }
}
