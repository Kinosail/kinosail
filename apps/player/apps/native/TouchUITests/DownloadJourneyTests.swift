import XCTest

final class DownloadJourneyTests: XCTestCase {
    private struct Fixture {
        let server: String
        let control: String
        let token: String
    }

    private struct ProxyState: Decodable {
        let bytes: Int
        let live: Int
    }

    @MainActor func testDownloadSurvivesOutageAndBackgroundAndNotifies() async throws {
        let fixture = try fixture()
        continueAfterFailure = true
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let springboard = XCUIApplication(bundleIdentifier: "com.apple.springboard")
        _ = try await proxy(fixture, "rate/2000000")
        _ = try await proxy(fixture, "mode/pass")
        app.launch()
        try await tourConnect(app, server: fixture.server, token: fixture.token, records: false)
        XCTAssertTrue(tourTab("Home", in: app).waitForExistence(timeout: 90), app.debugDescription)

        tourTab("Movies", in: app).tap()
        let movie = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Long Haul'")).firstMatch
        XCTAssertTrue(movie.waitForExistence(timeout: 30), app.debugDescription)
        movie.tap()
        let download = app.buttons["Download"].firstMatch
        XCTAssertTrue(download.waitForExistence(timeout: 20), app.debugDescription)
        download.tap()
        let original = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Original'")).firstMatch
        XCTAssertTrue(original.waitForExistence(timeout: 20), app.debugDescription)
        original.tap()
        app.buttons["Add to Downloads"].tap()
        let allow = springboard.buttons["Allow"]
        if allow.waitForExistence(timeout: 8) { allow.tap() }
        tourRecord("d1 added", app)

        if !app.navigationBars["Downloads"].waitForExistence(timeout: 10) {
            let status = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Download '")).firstMatch
            XCTAssertTrue(status.waitForExistence(timeout: 20), app.debugDescription)
            status.tap()
        }
        let row = rowElement("Long Haul", in: app)
        XCTAssertTrue(wait(for: row, containing: " of ", timeout: 90), app.debugDescription)
        sleep(6)
        tourRecord("d2 downloading", app)

        _ = try await proxy(fixture, "mode/drop")
        XCTAssertTrue(wait(for: row, containing: "reach your Server", timeout: 45), row.label)
        tourRecord("d3 server unreachable", app)
        sleep(75)
        tourRecord("d4 still waiting after outage", app)
        XCTAssertFalse(row.label.contains("Resume"), row.label)
        XCTAssertFalse(row.label.contains("interrupted"), row.label)
        let before = try await proxy(fixture, "mode/pass").bytes
        XCTAssertTrue(wait(for: row, containing: " left", timeout: 60), row.label)
        let resumed = try await proxy(fixture, "").bytes
        XCTAssertGreaterThan(resumed, before, "Transfer did not resume after the Server returned")
        tourRecord("d5 resumed automatically", app)

        let backgroundStart = try await proxy(fixture, "").bytes
        XCUIDevice.shared.press(.home)
        sleep(30)
        let backgroundEnd = try await proxy(fixture, "").bytes
        XCTAssertGreaterThan(backgroundEnd - backgroundStart, 30_000_000, "Background transfer moved \(backgroundEnd - backgroundStart) bytes in 30 s")
        app.activate()
        XCTAssertTrue(row.waitForExistence(timeout: 10), app.debugDescription)
        tourRecord("d6 after background", app)

        XCUIDevice.shared.press(.home)
        let banner = springboard.descendants(matching: .any).matching(NSPredicate(format: "label CONTAINS 'Long Haul'")).firstMatch
        XCTAssertTrue(banner.waitForExistence(timeout: 240), springboard.debugDescription)
        sleep(1)
        tourRecord("d7 ready notification", springboard)
        app.activate()
        let ready = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Long Haul'")).firstMatch
        XCTAssertTrue(ready.waitForExistence(timeout: 60), app.debugDescription)
        XCTAssertTrue(app.staticTexts["Ready to watch"].exists, app.debugDescription)
        tourRecord("d8 ready", app)
    }

    @MainActor func testConnectionBannerKeepsScreenTitleVisible() async throws {
        let fixture = try fixture()
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        _ = try await proxy(fixture, "rate/0")
        _ = try await proxy(fixture, "mode/pass")
        let restore = try XCTUnwrap(URL(string: "\(fixture.control)/mode/pass"))
        addTeardownBlock { _ = try? await URLSession.shared.data(from: restore) }
        app.launch()
        try await tourConnect(app, server: fixture.server, token: fixture.token, records: false)
        XCTAssertTrue(tourTab("Home", in: app).waitForExistence(timeout: 90), app.debugDescription)
        app.terminate()
        _ = try await proxy(fixture, "mode/drop")
        app.launch()
        let movies = tourTab("Movies", in: app)
        XCTAssertTrue(movies.waitForExistence(timeout: 30), app.debugDescription)
        movies.tap()
        let title = app.staticTexts["Can’t reach your Server"]
        let downloads = app.buttons["Downloads"].firstMatch
        XCTAssertTrue(title.waitForExistence(timeout: 60), app.debugDescription)
        XCTAssertTrue(downloads.waitForExistence(timeout: 5), app.debugDescription)
        let bar = app.navigationBars["Movies"]
        XCTAssertTrue(bar.waitForExistence(timeout: 10), app.debugDescription)
        tourRecord("c1 connection banner", app)
        let banner = title.frame.union(downloads.frame)
        XCTAssertFalse(banner.intersects(bar.frame), "Banner \(banner) covers navigation bar \(bar.frame)")
    }

    @MainActor private func rowElement(_ title: String, in app: XCUIApplication) -> XCUIElement {
        app.descendants(matching: .any).matching(NSPredicate(format: "label BEGINSWITH %@", title + ", ")).firstMatch
    }

    @MainActor private func wait(for element: XCUIElement, containing text: String, timeout: TimeInterval) -> Bool {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if element.exists, element.label.contains(text) { return true }
            usleep(500_000)
        }
        return false
    }

    private func proxy(_ fixture: Fixture, _ path: String) async throws -> ProxyState {
        let url = try XCTUnwrap(URL(string: "\(fixture.control)/\(path)"))
        let (data, _) = try await URLSession.shared.data(from: url)
        return try JSONDecoder().decode(ProxyState.self, from: data)
    }

    private func fixture() throws -> Fixture {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_DOWNLOAD_SERVER"], !server.isEmpty,
              let control = environment["KINOSAIL_DOWNLOAD_CONTROL"], !control.isEmpty,
              let token = environment["KINOSAIL_DOWNLOAD_TOKEN"], !token.isEmpty else {
            throw XCTSkip("Set KINOSAIL_DOWNLOAD_SERVER, KINOSAIL_DOWNLOAD_CONTROL and KINOSAIL_DOWNLOAD_TOKEN from scripts/download-fixture/up.sh.")
        }
        return Fixture(server: server, control: control, token: token)
    }
}
