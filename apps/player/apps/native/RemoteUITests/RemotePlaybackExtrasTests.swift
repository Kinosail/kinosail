import XCTest

final class RemotePlaybackExtrasTests: RemoteTestCase {
    private var remote: XCUIRemote { XCUIRemote.shared }

    @MainActor
    func testEpisodeMarkersCaptionsChaptersAndNextEpisode() throws {
        try XCTSkipUnless(ProcessInfo.processInfo.environment["KINOSAIL_TV_PLAYBACK_EXTRAS_FIXTURE"] == "1",
                          "Requires the navigation fixture with KINOSAIL_TV_PLAYBACK_EXTRAS_FIXTURE=1")
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        launchAtSearch(app)
        remote.press(.down)
        remote.press(.down)
        remote.press(.right)
        XCTAssertTrue(app.buttons["Shows"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        remote.press(.down)
        remote.press(.select)
        let play = app.buttons.matching(NSPredicate(format: "identifier BEGINSWITH 'show.next-play.'")).firstMatch
        XCTAssertTrue(play.waitForExistence(timeout: 10))
        XCTAssertTrue(play.hasFocus, "Play should take focus on the show page: \(focusedLabels(app))")
        let screen = app.frame.size
        remote.press(.select)

        XCTAssertTrue(waitForPrompt(screen, shown: true, timeout: 20), "The intro marker should offer Skip Intro")
        capture("skip intro offered")
        let skipped = Date()
        remote.press(.select)
        XCTAssertTrue(waitForPrompt(screen, shown: false, timeout: 5), "Skip Intro should go away after skipping")
        XCTAssertTrue(waitForPrompt(screen, shown: true, timeout: 20), "The credits marker should offer Next Episode")
        XCTAssertLessThan(Date().timeIntervalSince(skipped), 15, "Skipping should jump past the intro, not wait it out")
        capture("next episode offered")
        remote.press(.select)
        XCTAssertTrue(waitForPrompt(screen, shown: false, timeout: 5), "Next Episode should go away once chosen")
        XCTAssertTrue(waitForPrompt(screen, shown: true, timeout: 20), "The next episode should start with its own Skip Intro")
        capture("next episode intro")
        remote.press(.playPause)
        let nextTitle = app.alerts["Video player"].staticTexts["The Long Way Home"]
        XCTAssertTrue(nextTitle.waitForExistence(timeout: 20), "Next Episode should start S1 E2")
        record("next episode playing", app)
        remote.press(.playPause)

        let options = app.cells["Playback options"]
        XCTAssertTrue(options.waitForExistence(timeout: 20))
        remote.press(.select)
        remote.press(.up)
        for _ in 0..<12 where !options.hasFocus { remote.press(.right) }
        XCTAssertTrue(options.hasFocus, app.debugDescription)
        remote.press(.select)
        let subtitles = app.cells.matching(NSPredicate(format: "label BEGINSWITH 'Subtitles'")).firstMatch
        XCTAssertTrue(subtitles.waitForExistence(timeout: 10), app.debugDescription)
        XCTAssertTrue(subtitles.label.contains("Off"), "Subtitles start off, got \(subtitles.label)")
        let chapters = app.cells.matching(NSPredicate(format: "label BEGINSWITH 'Chapters'")).firstMatch
        XCTAssertTrue(chapters.exists && chapters.label.contains("3 chapters"), "Chapters show their count, got \(chapters.label)")
        record("episode playback options", app)
        for _ in 0..<8 where !subtitles.hasFocus { remote.press(.up) }
        XCTAssertTrue(subtitles.hasFocus, app.debugDescription)
        remote.press(.select)
        let english = app.cells.matching(NSPredicate(format: "label BEGINSWITH 'English'")).firstMatch
        XCTAssertTrue(english.waitForExistence(timeout: 10), app.debugDescription)
        for _ in 0..<4 where !english.hasFocus { remote.press(.down) }
        remote.press(.select)
        XCTAssertEqual(XCTWaiter.wait(for: [XCTNSPredicateExpectation(predicate: NSPredicate(format: "label CONTAINS 'English'"),
                                                                     object: subtitles)], timeout: 10), .completed,
                       "The Subtitles row shows the chosen track, got \(subtitles.label)")
        record("english subtitles chosen", app)
        for _ in 0..<8 where !chapters.hasFocus { remote.press(.down) }
        XCTAssertTrue(chapters.hasFocus, app.debugDescription)
        remote.press(.select)
        let journey = app.cells.matching(NSPredicate(format: "label BEGINSWITH 'Journey'")).firstMatch
        XCTAssertTrue(journey.waitForExistence(timeout: 10), app.debugDescription)
        record("chapter list", app)
        remote.press(.menu)
        XCTAssertTrue(chapters.waitForExistence(timeout: 10))
        remote.press(.menu)
        XCTAssertTrue(subtitles.waitForNonExistence(timeout: 10), "Back closes the options panel")
        XCTAssertTrue(options.waitForExistence(timeout: 10), "Playback stays available after the panel closes")
        remote.press(.menu)
        XCTAssertTrue(options.waitForNonExistence(timeout: 15), "Back must close the player")
        XCTAssertTrue(play.waitForExistence(timeout: 10), "Back returns to the show page")
        record("back to show", app)
    }

    private func waitForPrompt(_ screen: CGSize, shown: Bool, timeout: TimeInterval) -> Bool {
        let deadline = Date().addingTimeInterval(timeout)
        var shares: [Double] = []
        while Date() < deadline {
            let image = XCUIScreen.main.screenshot().image
            var light = 0, total = 0
            for x in stride(from: screen.width - 290, through: screen.width - 110, by: 6) {
                for y in stride(from: screen.height - 100, through: screen.height - 80, by: 6) {
                    total += 1
                    if samplePixel(image, x: x / screen.width, y: y / screen.height).prefix(3).allSatisfy({ $0 > 220 }) { light += 1 }
                }
            }
            shares.append(Double(light) / Double(total))
            if (shares.last ?? 0 > 0.25) == shown { return true }
            RunLoop.current.run(until: Date().addingTimeInterval(0.2))
        }
        capture("prompt \(shown ? "missing" : "stuck") \(shares.suffix(8))")
        return false
    }

    private func capture(_ name: String) {
        let attachment = XCTAttachment(screenshot: XCUIScreen.main.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }
}
