import XCTest

extension RemoteArtworkStateTests {
    // Isolated production-view fixture with public traits, not system Settings.
    // XCUI uses the same accessibility/focus path as the accepted normal case.
    @MainActor
    func testAccessibilityXXXLDetailActionsAlignAndPreserveNavigation() async throws {
        try requireCase("large-text-fixture")
        try await state("loaded")
        let env = ProcessInfo.processInfo.environment
        let identity = try XCTUnwrap(env["KINOSAIL_TV_QUALITY_FIXTURE_ID"])
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player.large-text-fixture")
        app.launchEnvironment = ["KINOSAIL_TV_LARGE_TEXT_FIXTURE": "1",
                                 "KINOSAIL_TV_QUALITY_FIXTURE_MODE": "loaded",
                                 "KINOSAIL_TV_QUALITY_FIXTURE_ID": identity]
        app.launch()
        defer { app.terminate(); XCTAssertEqual(app.state, .notRunning) }
        let readback = app.staticTexts["fixture.large-text.readback"]
        let expected = "\(identity) root=accessibility5 detail=accessibility5 UIKit=UICTContentSizeCategoryAccessibilityXXXL"
        let ready = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            readback.exists && readback.label == expected
                && app.staticTexts["After the Rain"].exists && app.buttons["Resume"].exists
        }, object: nil)
        let loaded = await XCTWaiter.fulfillment(of: [ready], timeout: 10)
        if loaded != .completed { record("detail-large-text-load-failure", app) }
        XCTAssertEqual(loaded, .completed, "Actual root and detail subtree must receive the requested category")
        let resume = app.buttons["Resume"]
        let myList = app.buttons["My List"]
        let bookmarks = app.buttons["Bookmarks"]
        let remove = app.buttons["Remove from Continue watching"]
        XCTAssertEqual(resume.identifier, "detail.play.movie-004")
        for label in ["Resume", "My List", "Bookmarks", "Remove from Continue watching"] {
            XCTAssertEqual(app.buttons.matching(NSPredicate(format: "label == %@", label)).count, 1)
        }
        func focused(_ element: XCUIElement, _ name: String) {
            waitForFocus(element, in: app)
            record("detail-large-text-\(name)", app)
            XCTAssertEqual(readback.label, expected)
            let frame = element.frame
            XCTAssertTrue([frame.minX, frame.minY, frame.maxX, frame.maxY].allSatisfy(\.isFinite))
            XCTAssertGreaterThan(frame.width, 0)
            XCTAssertGreaterThan(frame.height, 0)
            XCTAssertTrue(app.frame.contains(frame), "Focused \(name) must be fully visible: \(frame)")
        }
        let remote = XCUIRemote.shared
        focused(resume, "resume-focused")
        remote.press(.down)
        focused(myList, "list-focused")
        let stacked = abs(myList.frame.midY - bookmarks.frame.midY) > 1
        XCTAssertFalse(bookmarks.hasFocus)
        XCTAssertFalse(remove.hasFocus)
        let restingBookmarks = bookmarks.frame
        let restingRemoveBesideBookmarks = remove.frame
        remote.press(stacked ? .down : .right)
        focused(bookmarks, "bookmarks-focused")
        XCTAssertFalse(resume.hasFocus)
        XCTAssertFalse(myList.hasFocus)
        XCTAssertFalse(remove.hasFocus)
        for frame in [resume.frame, myList.frame, remove.frame, restingBookmarks] {
            XCTAssertTrue([frame.minX, frame.minY, frame.maxX, frame.maxY].allSatisfy(\.isFinite))
            XCTAssertGreaterThan(frame.width, 0)
            XCTAssertGreaterThan(frame.height, 0)
        }
        XCTAssertEqual(resume.frame.height, myList.frame.height, accuracy: 1)
        XCTAssertEqual(resume.frame.height, remove.frame.height, accuracy: 1)
        XCTAssertEqual(resume.frame.height, restingBookmarks.height, accuracy: 1)
        XCTAssertEqual(resume.frame.minX, myList.frame.minX, accuracy: 1)
        if stacked {
            XCTAssertEqual(myList.frame.minX, remove.frame.minX, accuracy: 1)
            XCTAssertEqual(myList.frame.minX, restingBookmarks.minX, accuracy: 1)
        } else {
            XCTAssertEqual(myList.frame.minY, remove.frame.minY, accuracy: 1)
            XCTAssertEqual(restingBookmarks.minY, restingRemoveBesideBookmarks.minY, accuracy: 1)
        }
        XCTAssertGreaterThan(resume.frame.height, 92, "Large Text must enlarge the normal 92-point resting control")
        remote.press(stacked ? .down : .right)
        focused(remove, "remove-focused")
        remote.press(.up)
        if stacked {
            waitForFocus(bookmarks, in: app)
            remote.press(.up)
            waitForFocus(myList, in: app)
            remote.press(.up)
        }
        focused(resume, "resume-return")
        verifyPrimaryVisible(app)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        print("LARGE_TEXT_FIXTURE accepted=\(expected) stacked=\(stacked) restingHeight=\(myList.frame.height)")
    }
}
