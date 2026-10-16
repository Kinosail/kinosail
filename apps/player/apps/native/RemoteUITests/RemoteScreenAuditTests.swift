import XCTest

final class RemoteScreenAuditTests: RemoteTestCase {
    private var remote: XCUIRemote { XCUIRemote.shared }

    @MainActor private func launch() -> XCUIApplication {
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launch()
        XCTAssertTrue(app.staticTexts["Continue watching"].waitForExistence(timeout: 20))
        sleep(2)
        return app
    }

    @MainActor private func press(_ button: XCUIRemote.Button, _ count: Int = 1, then name: String? = nil, in app: XCUIApplication) {
        for _ in 0..<count { remote.press(button); usleep(450_000) }
        if let name { sleep(1); capture(name, app) }
    }

    @MainActor private func capture(_ name: String, _ app: XCUIApplication) {
        let attachment = XCTAttachment(screenshot: XCUIScreen.main.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }

    @MainActor private func openBrowse(_ index: Int, _ app: XCUIApplication) {
        press(.down, 2, in: app)
        press(.right, index, in: app)
        press(.select, in: app)
        sleep(3)
    }

    @MainActor func testAudit01Home() {
        let app = launch()
        capture("home-01-launch", app)
        press(.down, then: "home-02-continue", in: app)
        press(.right, then: "home-03-continue-right", in: app)
        press(.down, then: "home-04-browse", in: app)
        for index in 5...12 { press(.down, then: "home-\(String(format: "%02d", index))-down", in: app) }
        press(.up, 12, then: "home-13-back-top", in: app)
    }

    @MainActor func testAudit02Movies() {
        let app = launch()
        openBrowse(0, app)
        capture("movies-01-grid", app)
        press(.down, then: "movies-02-grid-down", in: app)
        press(.down, 3, then: "movies-03-grid-deeper", in: app)
        press(.up, 8, then: "movies-04-grid-top-controls", in: app)
        press(.down, in: app)
        press(.select, then: "movies-05-detail", in: app)
        sleep(2)
        capture("movies-06-detail-settled", app)
        press(.right, then: "movies-07-detail-right", in: app)
        press(.down, then: "movies-08-detail-down", in: app)
        press(.down, then: "movies-09-detail-down2", in: app)
        press(.down, then: "movies-10-detail-down3", in: app)
        press(.menu, then: "movies-11-back-to-grid", in: app)
    }

    @MainActor func testAudit03Shows() {
        let app = launch()
        openBrowse(1, app)
        capture("shows-01-grid", app)
        press(.down, in: app)
        press(.select, then: "shows-02-show", in: app)
        sleep(2)
        capture("shows-03-show-settled", app)
        press(.down, then: "shows-04-down", in: app)
        press(.down, then: "shows-05-down2", in: app)
        press(.right, then: "shows-06-right", in: app)
        press(.down, then: "shows-07-down3", in: app)
        press(.select, then: "shows-08-select", in: app)
        sleep(2)
        capture("shows-09-select-settled", app)
    }

    @MainActor func testAudit04Music() {
        let app = launch()
        openBrowse(2, app)
        capture("music-01", app)
        press(.down, then: "music-02-down", in: app)
        press(.select, then: "music-03-select", in: app)
        sleep(2)
        capture("music-04-settled", app)
        press(.down, then: "music-05-down", in: app)
    }

    @MainActor func testAudit05Audiobooks() {
        let app = launch()
        openBrowse(3, app)
        capture("books-01", app)
        press(.down, then: "books-02-down", in: app)
        press(.select, then: "books-03-select", in: app)
        sleep(2)
        capture("books-04-settled", app)
    }

    @MainActor func testAudit06Photos() {
        let app = launch()
        openBrowse(4, app)
        capture("photos-01", app)
        press(.down, then: "photos-02-down", in: app)
        press(.select, then: "photos-03-select", in: app)
        sleep(2)
        capture("photos-04-settled", app)
    }

    @MainActor func testAudit07Search() {
        let app = launch()
        press(.select, then: "search-01", in: app)
        let field = app.searchFields.firstMatch
        if field.waitForExistence(timeout: 5) {
            field.typeText("moon")
            sleep(3)
            capture("search-02-typed", app)
        }
        press(.down, 3, then: "search-03-results", in: app)
        press(.right, then: "search-04-results-right", in: app)
    }

    @MainActor func testAudit08Settings() {
        let app = launch()
        press(.right, in: app)
        press(.select, then: "settings-01", in: app)
        for index in 2...9 { press(.down, then: "settings-\(String(format: "%02d", index))", in: app) }
        press(.up, 8, in: app)
        press(.select, then: "settings-10-first-row", in: app)
        press(.menu, then: "settings-11-back", in: app)
    }

    @MainActor func testAudit09Playback() {
        let app = launch()
        press(.down, in: app)
        press(.select, in: app)
        sleep(6)
        capture("play-01", app)
        press(.select, then: "play-02-controls", in: app)
        press(.down, then: "play-03-info", in: app)
        press(.menu, in: app)
        press(.up, then: "play-04-up", in: app)
        press(.menu, 2, then: "play-05-exit", in: app)
    }

    @MainActor func testAudit10MovieDetailFromHome() {
        let app = launch()
        press(.down, 4, in: app)
        press(.select, then: "detail-01-from-recent", in: app)
        sleep(2)
        capture("detail-02-settled", app)
        press(.down, 2, then: "detail-03-down", in: app)
        press(.down, 2, then: "detail-04-down", in: app)
    }
}
