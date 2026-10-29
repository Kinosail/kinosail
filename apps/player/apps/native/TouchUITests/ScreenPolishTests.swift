import XCTest

final class ScreenPolishTests: XCTestCase {
    @MainActor private func connectedApp(textSize: String = "UICTContentSizeCategoryL") async throws -> XCUIApplication {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_DESIGN_TOUR_SERVER"], !server.isEmpty else {
            throw XCTSkip("Set KINOSAIL_DESIGN_TOUR_SERVER to a populated test Server.")
        }
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launchArguments += ["-UIPreferredContentSizeCategoryName", textSize]
        app.launch()
        try await tourConnect(app, server: server, token: environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? "", records: false)
        XCTAssertTrue(tourTab("Home", in: app).waitForExistence(timeout: 90), app.debugDescription)
        return app
    }

    @MainActor private func reveal(_ element: XCUIElement, in app: XCUIApplication) {
        for _ in 0..<5 where !element.isHittable { app.swipeUp() }
    }

    @MainActor func testLibraryFiltersStayOnScreenAtLargestTextSize() async throws {
        let app = try await connectedApp(textSize: "UICTContentSizeCategoryAccessibilityXXXL")
        tourTab("More", in: app).tap()
        let books = app.buttons["Books"].firstMatch
        XCTAssertTrue(books.waitForExistence(timeout: 10), app.debugDescription)
        reveal(books, in: app)
        books.tap()
        XCTAssertTrue(app.buttons["Library, Ebooks & comics"].waitForExistence(timeout: 10), app.debugDescription)
        sleep(2)
        tourRecord("p1 books filters largest text", app)
        let screen = app.windows.firstMatch.frame
        for filter in [app.buttons["Library, Ebooks & comics"], app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Sort, '")).firstMatch] {
            XCTAssertGreaterThanOrEqual(filter.frame.minX, screen.minX, "\(filter.label) starts off screen: \(filter.frame)")
            XCTAssertLessThanOrEqual(filter.frame.maxX, screen.maxX, "\(filter.label) ends off screen: \(filter.frame)")
        }
    }

    @MainActor func testEbookTextWrapsInsideScreenAtLargestTextSize() async throws {
        let app = try await connectedApp(textSize: "UICTContentSizeCategoryAccessibilityXXXL")
        tourTab("More", in: app).tap()
        let books = app.buttons["Books"].firstMatch
        XCTAssertTrue(books.waitForExistence(timeout: 10), app.debugDescription)
        reveal(books, in: app)
        books.tap()
        let book = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Example EPUB Book'")).firstMatch
        XCTAssertTrue(book.waitForExistence(timeout: 20), app.debugDescription)
        reveal(book, in: app)
        book.tap()
        let read = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Read'")).firstMatch
        XCTAssertTrue(read.waitForExistence(timeout: 20), app.debugDescription)
        read.tap()
        let text = app.webViews.staticTexts.firstMatch
        XCTAssertTrue(text.waitForExistence(timeout: 30), app.debugDescription)
        sleep(2)
        tourRecord("p3 ebook largest text", app)
        let screen = app.windows.firstMatch.frame
        let lines = app.webViews.staticTexts.allElementsBoundByIndex.filter { $0.frame.minY < screen.maxY && !$0.label.isEmpty }
        XCTAssertFalse(lines.isEmpty, app.debugDescription)
        for line in lines {
            XCTAssertLessThanOrEqual(line.frame.maxX, screen.maxX + 1, "\"\(line.label)\" runs past the screen edge: \(line.frame)")
        }
    }

    @MainActor func testLibraryUsesTheSameSymbolsAsMore() async throws {
        let app = try await connectedApp()
        tourTab("More", in: app).tap()
        let names = ["My List", "Collections"]
        var symbols: [String: String] = [:]
        for name in names {
            let row = app.buttons[name].firstMatch
            XCTAssertTrue(row.waitForExistence(timeout: 10), app.debugDescription)
            reveal(row, in: app)
            symbols[name] = row.images.firstMatch.identifier
        }
        app.swipeDown()
        app.buttons["Library"].firstMatch.tap()
        XCTAssertTrue(app.navigationBars["Library"].waitForExistence(timeout: 10), app.debugDescription)
        for name in names {
            let row = app.buttons[name].firstMatch
            XCTAssertTrue(row.waitForExistence(timeout: 10), app.debugDescription)
            reveal(row, in: app)
            XCTAssertEqual(row.images.firstMatch.identifier, symbols[name], "\(name) symbol differs from More")
        }
    }

    @MainActor func testResetTabsIsAvailableOnlyAfterTabsChange() async throws {
        let app = try await connectedApp()
        tourTab("More", in: app).tap()
        let customize = app.buttons["Customize tabs"].firstMatch
        XCTAssertTrue(customize.waitForExistence(timeout: 10), app.debugDescription)
        customize.tap()
        let reset = app.buttons["Reset to default tabs"]
        XCTAssertTrue(reset.waitForExistence(timeout: 10), app.debugDescription)
        reveal(reset, in: app)
        XCTAssertFalse(reset.isEnabled, "Reset is offered while the default tabs are already in use")
        app.swipeDown()
        let remove = app.buttons["Remove Movies from tabs"]
        XCTAssertTrue(remove.waitForExistence(timeout: 5), app.debugDescription)
        remove.tap()
        reveal(reset, in: app)
        XCTAssertTrue(reset.isEnabled, app.debugDescription)
        reset.tap()
        XCTAssertTrue(app.buttons["Remove Movies from tabs"].waitForExistence(timeout: 5), app.debugDescription)
        XCTAssertFalse(reset.isEnabled, app.debugDescription)
    }

    @MainActor func testPlaybackOptionsDoneSitsWithOtherSheetConfirmations() async throws {
        let app = try await connectedApp()
        tourTab("Movies", in: app).tap()
        let movie = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Arrival'")).firstMatch
        XCTAssertTrue(movie.waitForExistence(timeout: 20), app.debugDescription)
        movie.tap()
        let play = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Play' OR label BEGINSWITH 'Resume'")).firstMatch
        XCTAssertTrue(play.waitForExistence(timeout: 20), app.debugDescription)
        play.tap()
        let options = app.buttons["Playback options"]
        XCTAssertTrue(options.waitForExistence(timeout: 30), app.debugDescription)
        if !options.isHittable { app.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.45)).tap() }
        options.tap()
        let bar = app.navigationBars["Playback options"]
        XCTAssertTrue(bar.waitForExistence(timeout: 10), app.debugDescription)
        let done = bar.buttons["Done"]
        XCTAssertTrue(done.waitForExistence(timeout: 5), app.debugDescription)
        tourRecord("p2 playback options", app)
        XCTAssertGreaterThan(done.frame.midX, bar.frame.midX, "Done is at the leading edge: \(done.frame) in \(bar.frame)")
        done.tap()
    }
}
