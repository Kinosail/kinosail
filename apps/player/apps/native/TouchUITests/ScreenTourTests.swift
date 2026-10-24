import XCTest

final class ScreenTourTests: XCTestCase {
    @MainActor func testCaptureConsistencyTour() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_DESIGN_TOUR_SERVER"], !server.isEmpty else {
            throw XCTSkip("Set KINOSAIL_DESIGN_TOUR_SERVER to a populated test Server.")
        }
        continueAfterFailure = true
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        try await tourConnect(app, server: server, token: environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? "", records: false)
        let home = tourTab("Home", in: app)
        XCTAssertTrue(home.waitForExistence(timeout: 90), app.debugDescription)
        let shows = tourTab("Shows", in: app)
        if shows.exists {
            shows.tap()
            let show = app.buttons.matching(NSPredicate(format: "label CONTAINS 'Example Show'")).firstMatch
            XCTAssertTrue(show.waitForExistence(timeout: 20), app.debugDescription)
            show.tap()
            sleep(4)
            tourRecord("c1 show", app)
            app.swipeUp()
            sleep(1)
            tourRecord("c2 show lower", app)
            app.navigationBars.buttons.firstMatch.tap()
        }
        let movies = tourTab("Movies", in: app)
        if movies.exists {
            movies.tap()
            let movie = app.buttons.matching(NSPredicate(format: "label CONTAINS 'Arrival'")).firstMatch
            XCTAssertTrue(movie.waitForExistence(timeout: 20), app.debugDescription)
            movie.tap()
            sleep(4)
            tourRecord("c3 movie", app)
            app.navigationBars.buttons.firstMatch.tap()
        }
        for (index, (name, item)) in [("Collections", "Example Collection"), ("Music", "Example Album")].enumerated() {
            tourReveal(home, in: app)
            tourTab("More", in: app).tap()
            let link = app.buttons[name].firstMatch
            guard link.waitForExistence(timeout: 5) else { continue }
            link.tap()
            sleep(3)
            tourRecord("c\(4 + index * 2) \(name.lowercased())", app)
            let entry = app.buttons.matching(NSPredicate(format: "label CONTAINS %@", item)).firstMatch
            if entry.waitForExistence(timeout: 10) {
                entry.tap()
                sleep(3)
                tourRecord("c\(5 + index * 2) \(item.lowercased())", app)
            }
            tourTab("More", in: app).tap()
            tourTab("More", in: app).tap()
        }
    }

    @MainActor func testCaptureEveryScreen() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_DESIGN_TOUR_SERVER"], !server.isEmpty else {
            throw XCTSkip("Set KINOSAIL_DESIGN_TOUR_SERVER to a populated test Server.")
        }
        continueAfterFailure = true
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        try await tourConnect(app, server: server, token: environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? "", records: false)
        let home = tourTab("Home", in: app)
        XCTAssertTrue(home.waitForExistence(timeout: 90), app.debugDescription)
        let pages: [(String, [String])] = [
            ("My List", []), ("Library", []), ("Music", []), ("Audiobooks", ["first"]), ("Books", ["first"]),
            ("Photos", ["first"]), ("Downloads", []),
            ("Settings", ["Playback", "Downloads", "Reader", "Customize tabs", "Connect a TV"]),
        ]
        for (index, (name, children)) in pages.enumerated() {
            tourReveal(home, in: app)
            let more = tourTab("More", in: app)
            more.tap()
            more.tap()
            let link = app.buttons[name].firstMatch
            guard link.waitForExistence(timeout: 5) else { continue }
            link.tap()
            sleep(3)
            tourRecord(String(format: "e%02d %@", index * 10, name.lowercased()), app)
            for (offset, child) in children.enumerated() {
                let target = child == "first"
                    ? app.scrollViews.firstMatch.buttons.element(boundBy: 0)
                    : app.buttons.matching(NSPredicate(format: "label BEGINSWITH %@", child)).element(boundBy: 0)
                guard target.waitForExistence(timeout: 5) else { continue }
                target.tap()
                sleep(4)
                tourRecord(String(format: "e%02d %@ %@", index * 10 + offset + 1, name.lowercased(), child.lowercased()), app)
                if app.buttons["Close player"].exists { app.buttons["Close player"].tap() }
                let back = app.navigationBars.buttons.firstMatch
                if back.exists && back.isHittable { back.tap() } else { app.swipeDown() }
                sleep(1)
            }
        }
    }

    @MainActor func testTabBarCollapsesOnScrollDownAndExpandsOnScrollUp() async throws {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_DESIGN_TOUR_SERVER"], !server.isEmpty else {
            throw XCTSkip("Set KINOSAIL_DESIGN_TOUR_SERVER to a populated test Server.")
        }
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        try await tourConnect(app, server: server, token: environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? "", records: false)
        let home = tourTab("Home", in: app)
        XCTAssertTrue(home.waitForExistence(timeout: 90), app.debugDescription)
        let other = tourTab("Shows", in: app)
        sleep(3)
        tourRecord("t1 tab bar at top", app)
        XCTAssertTrue(other.isHittable)
        guard UIDevice.current.userInterfaceIdiom == .phone else { return }
        app.swipeUp()
        sleep(1)
        tourRecord("t2 tab bar collapsed", app)
        XCTAssertFalse(other.isHittable, "Tab bar stays expanded after scrolling down")
        let middle = app.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.4))
        middle.press(forDuration: 0.05, thenDragTo: app.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.8)))
        sleep(1)
        tourRecord("t3 tab bar expanded", app)
        XCTAssertTrue(other.isHittable, "Tab bar stays collapsed after scrolling up")
        app.swipeUp()
        app.swipeUp()
        sleep(1)
        tourRecord("t4 tab bar collapsed again", app)
        XCTAssertFalse(other.isHittable, "Tab bar does not collapse again after expanding")
    }
}
