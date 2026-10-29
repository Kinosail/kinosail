import XCTest

final class IPadWindowTests: XCTestCase {
    @MainActor private func connectedApp(orientation: UIDeviceOrientation) async throws -> XCUIApplication {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_DESIGN_TOUR_SERVER"], !server.isEmpty else {
            throw XCTSkip("Set KINOSAIL_DESIGN_TOUR_SERVER to a populated test Server.")
        }
        guard UIDevice.current.userInterfaceIdiom == .pad else { throw XCTSkip("iPad windows only.") }
        XCUIDevice.shared.orientation = orientation
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        try await tourConnect(app, server: server, token: environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? "", records: false)
        XCTAssertTrue(tab("Home", in: app).waitForExistence(timeout: 90), app.debugDescription)
        return app
    }

    @MainActor private func tab(_ name: String, in app: XCUIApplication) -> XCUIElement {
        let bar = tourTab(name, in: app)
        if bar.exists { return bar }
        let row = app.cells.containing(.staticText, identifier: name).firstMatch
        return row.exists ? row : app.staticTexts.matching(NSPredicate(format: "label == %@", name)).firstMatch
    }

    @MainActor private func resize(_ app: XCUIApplication, width: CGFloat) {
        let screen = XCUIApplication(bundleIdentifier: "com.apple.springboard").frame
        let window = app.windows.firstMatch.frame
        let corner = XCUIApplication(bundleIdentifier: "com.apple.springboard")
            .coordinate(withNormalizedOffset: .zero)
            .withOffset(CGVector(dx: window.maxX - 3, dy: window.maxY - 3))
        let target = corner.withOffset(CGVector(dx: window.minX + width - window.maxX, dy: min(0, screen.maxY - 8 - window.maxY)))
        corner.press(forDuration: 0.4, thenDragTo: target)
        sleep(2)
    }

    @MainActor private func assertContentFits(_ name: String, _ app: XCUIApplication) {
        let window = app.windows.firstMatch.frame
        for text in app.staticTexts.allElementsBoundByIndex where text.exists && !text.frame.isEmpty {
            let frame = text.frame
            guard frame.maxY > window.minY, frame.minY < window.maxY, !text.label.isEmpty else { continue }
            guard frame.minX >= window.minX - 1, frame.maxX <= window.maxX + 1 else {
                if text.isHittable { XCTFail("\(name): \"\(text.label)\" runs outside the window \(window): \(frame)") }
                continue
            }
        }
    }

    @MainActor private func tour(_ app: XCUIApplication, label: String) {
        tourRecord("\(label) home", app)
        assertContentFits("\(label) home", app)
        app.swipeUp()
        sleep(1)
        tourRecord("\(label) home lower", app)
        for (name, item) in [("Movies", "Delta"), ("Shows", "Example Show")] {
            var link = tab(name, in: app)
            if !link.exists { tab("Home", in: app).tap(); app.swipeDown(); link = tab(name, in: app) }
            guard link.waitForExistence(timeout: 5) else { XCTFail("\(label): no \(name) tab\n\(app.debugDescription)"); continue }
            link.tap()
            sleep(2)
            tourRecord("\(label) \(name.lowercased())", app)
            assertContentFits("\(label) \(name)", app)
            let card = app.buttons.matching(NSPredicate(format: "label CONTAINS %@", item)).firstMatch
            guard card.waitForExistence(timeout: 15) else { continue }
            card.tap()
            sleep(3)
            tourRecord("\(label) \(name.lowercased()) detail", app)
            assertContentFits("\(label) \(name) detail", app)
            app.swipeUp()
            sleep(1)
            tourRecord("\(label) \(name.lowercased()) detail lower", app)
            tourBack(app)
        }
    }

    @MainActor func testNarrowWindowsKeepContentInside() async throws {
        let app = try await connectedApp(orientation: .portrait)
        for width: CGFloat in [507, 375, 320] {
            resize(app, width: width)
            let label = "w\(Int(width))-\(Int(app.windows.firstMatch.frame.width))"
            tour(app, label: label)
            tab("Home", in: app).tap()
        }
        resize(app, width: 2000)
    }

    @MainActor func testLandscapeTour() async throws {
        let app = try await connectedApp(orientation: .landscapeLeft)
        tour(app, label: "landscape")
        for name in ["Search", "More"] {
            let link = tab(name, in: app)
            guard link.waitForExistence(timeout: 5) else { XCTFail("landscape: no \(name)"); continue }
            link.tap()
            sleep(2)
            tourRecord("landscape \(name.lowercased())", app)
            assertContentFits("landscape \(name)", app)
        }
        XCUIDevice.shared.orientation = .portrait
    }
}
