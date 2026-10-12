import XCTest

extension RemoteArtworkStateTests {
    // Real focused controls own their visual treatment; geometry is compared
    // while the measured controls are unfocused. Widths follow their labels.
    @MainActor
    func testDetailActionsShareCapsuleGeometryAndPreserveNavigation() async throws {
        try requireCase("appearance")
        try await state("loaded")
        let app = openMovie()
        waitForDetail(app, title: "After the Rain")
        let remote = XCUIRemote.shared
        let resume = app.buttons["Resume"]
        let myList = app.buttons["My List"]
        let bookmarks = app.buttons["Bookmarks"]
        let remove = app.buttons["Remove from Continue watching"]
        XCTAssertEqual(resume.identifier, "detail.play.movie-004")
        XCTAssertTrue(remove.exists)
        record("detail-actions-resume-focused", app)

        remote.press(.down)
        waitForFocus(myList, in: app)
        record("detail-actions-list-focused", app)
        remote.press(.right)
        waitForFocus(bookmarks, in: app)
        record("detail-actions-bookmarks-focused", app)

        // These three are all unfocused, so native focus scaling does not
        // change the bounds used to check the shared height and leading edge.
        XCTAssertFalse(resume.hasFocus)
        XCTAssertFalse(myList.hasFocus)
        XCTAssertFalse(remove.hasFocus)
        XCTAssertEqual(resume.frame.height, myList.frame.height, accuracy: 1)
        XCTAssertEqual(resume.frame.height, remove.frame.height, accuracy: 1)
        XCTAssertEqual(resume.frame.minX, myList.frame.minX, accuracy: 1)
        XCTAssertEqual(myList.frame.minY, remove.frame.minY, accuracy: 1)

        remote.press(.right)
        waitForFocus(remove, in: app)
        record("detail-actions-remove-focused", app)
        remote.press(.up)
        waitForFocus(resume, in: app)
        record("detail-actions-resume-return", app)
        verifyPrimaryVisible(app)
        remote.press(.menu)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
    }
}
