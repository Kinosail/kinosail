import UIKit
import XCTest

final class RemoteLayoutPolishTests: RemoteTestCase {
    private var remote: XCUIRemote { XCUIRemote.shared }

    @MainActor
    func testFocusingLongAndShortTitlesKeepsTheContinueWatchingRailStill() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        launchAtSearch(app)
        let heading = app.staticTexts["Continue watching"]
        XCTAssertTrue(heading.waitForExistence(timeout: 15))
        remote.press(.down)
        settle()
        let resting = heading.frame.minY
        record("rail with a short spotlight title", app)
        for _ in 0..<3 {
            remote.press(.right)
            settle()
            XCTAssertEqual(heading.frame.minY, resting, accuracy: 1,
                           "Focusing \(focusedLabels(app)) must not move the rail under the spotlight")
        }
        record("rail after long and short spotlight titles", app)
    }

    @MainActor
    func testHomeShelvesFadeUnderTheTopBarInsteadOfACutLine() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        launchAtSearch(app)
        let edge = app.buttons["Search"].frame.maxY + 12
        var crossing: XCUIElement?
        for _ in 0..<8 where crossing == nil {
            remote.press(.down)
            settle()
            crossing = app.buttons.allElementsBoundByIndex.first { card in
                let frame = card.frame
                return frame.width > 200 && frame.width < 260 && frame.minY < edge - 10 && frame.maxY > edge + 80
            }
        }
        guard let card = crossing else { return XCTFail("A poster should scroll across the top bar edge") }
        record("poster under the top bar", app)
        let image = app.screenshot().image
        let top = app.buttons["Search"].frame.maxY
        let rows = stride(from: top, through: top + 56, by: 2).map { y in
            (y, stride(from: card.frame.minX + 8, through: card.frame.maxX - 8, by: 6).map { x in
                brightness(samplePixel(image, x: x / app.frame.width, y: y / app.frame.height))
            }.max() ?? 0)
        }
        let peak = rows.map(\.1).max() ?? 0
        XCTAssertGreaterThan(peak, 30, "The fixture poster should be visible below the edge")
        let faint = rows.first { $0.1 * 10 >= peak }?.0 ?? top
        let full = rows.first { $0.1 * 10 >= peak * 9 }?.0 ?? top
        XCTAssertGreaterThanOrEqual(full - faint, 16, "Content should fade out at the top bar edge, not end in a hard cut: \(rows)")
    }

    @MainActor
    func testSeasonPillsAlignWithEpisodesAndShowAVisibleFocus() {
        continueAfterFailure = false
        let app = openShow()
        let season = app.buttons["show.season-1"]
        let episodes = app.staticTexts["Episodes"]
        XCTAssertTrue(season.waitForExistence(timeout: 10))
        XCTAssertEqual(season.frame.minX, episodes.frame.minX, accuracy: 4,
                       "Season pills should share the leading edge of the Episodes shelf")
        remote.press(.down)
        settle()
        XCTAssertTrue(season.hasFocus, "Down from Play should reach the selected season; focus: \(focusedLabels(app))")
        record("focused selected season", app)
        let frame = season.frame.insetBy(dx: 30, dy: 12)
        XCTAssertGreaterThan(lightShare(app, in: frame), 0.5,
                             "A focused season needs the white focus fill used by the other capsules")
        let other = app.buttons["show.season-2"]
        XCTAssertLessThanOrEqual(season.frame.maxX + 8, other.frame.minX,
                                 "The focused season must not touch the next season")
    }

    @MainActor
    func testEpisodeShelfRunsToTheBottomOfTheScreen() {
        continueAfterFailure = false
        let app = openShow()
        let card = app.buttons.matching(NSPredicate(format: "label CONTAINS 'First Light'")).firstMatch
        XCTAssertTrue(card.waitForExistence(timeout: 10))
        let bottom = app.frame.height - 30
        XCTAssertGreaterThan(card.frame.maxY, bottom, "The fixture places the episode card across the bottom edge")
        let visible = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let color = self.samplePixel(app.screenshot().image, x: card.frame.midX / app.frame.width, y: bottom / app.frame.height)
            return Int(color[2]) > Int(color[1]) + 25
        }, object: nil)
        let result = XCTWaiter.wait(for: [visible], timeout: 10)
        record("episode shelf at the bottom edge", app)
        XCTAssertEqual(result, .completed, "Episode artwork should continue under the bottom safe area like Home shelves")
    }

    @MainActor
    func testBrowseTilesShowAVisibleFocusOutline() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        launchAtSearch(app)
        remote.press(.down)
        remote.press(.down)
        let movies = app.buttons["Movies"]
        XCTAssertTrue(movies.hasFocus)
        remote.press(.right)
        let shows = app.buttons["Shows"]
        XCTAssertTrue(shows.hasFocus)
        settle()
        record("focused Browse tile", app)
        let frame = shows.frame
        let lightRows = (-8...10).filter { offset in
            [0.3, 0.5, 0.7].allSatisfy { fraction in
                let color = samplePixel(app.screenshot().image, x: (frame.minX + frame.width * fraction) / app.frame.width,
                                  y: (frame.minY + CGFloat(offset)) / app.frame.height)
                return color.prefix(3).allSatisfy { $0 > 180 }
            }
        }
        XCTAssertGreaterThanOrEqual(lightRows.count, 2, "The focused Browse tile needs the bright outline used by media cards")
        let unfocused = movies.frame
        let quietRows = (-8...10).filter { offset in
            let color = samplePixel(app.screenshot().image, x: unfocused.midX / app.frame.width,
                              y: (unfocused.minY + CGFloat(offset)) / app.frame.height)
            return color.prefix(3).allSatisfy { $0 > 180 }
        }
        XCTAssertTrue(quietRows.isEmpty, "Only the focused tile should carry the outline")
    }

    @MainActor
    func testSettingsExplanationsAreFootersNotRows() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        launchAtSearch(app)
        remote.press(.right)
        remote.press(.select)
        XCTAssertTrue(app.buttons["Change Server"].waitForExistence(timeout: 10))
        var seen = Set<String>()
        let collect = {
            for start in ["Everyone using this Apple TV", "Show Continue watching, My List"]
            where app.descendants(matching: .any).matching(NSPredicate(format: "label BEGINSWITH %@", start)).firstMatch.exists {
                XCTAssertEqual(app.cells.containing(NSPredicate(format: "label BEGINSWITH %@", start)).count, 0,
                               "\(start)… should be a section footer, not a list row")
                seen.insert(start)
            }
        }
        for _ in 0..<10 {
            collect()
            if focusedLabels(app).contains(where: { $0.contains("Sign out") }) { break }
            remote.press(.down)
        }
        settle()
        collect()
        record("settings footers", app)
        XCTAssertEqual(seen.count, 2, "Every Settings explanation should be reachable: \(seen)")
        XCTAssertEqual(app.cells.containing(NSPredicate(format: "label BEGINSWITH 'Kinosail plays directly'")).count, 0,
                       "The session note should be a footer, not a list row")
        let signOut = app.cells["Sign out"].frame
        let note = CGRect(x: signOut.minX + 20, y: signOut.maxY + 16, width: signOut.width / 2, height: 40)
        let image = app.screenshot().image
        let text = stride(from: note.minX, through: note.maxX, by: 3).flatMap { x in
            stride(from: note.minY, through: note.maxY, by: 3).map { y in brightness(samplePixel(image, x: x / app.frame.width, y: y / app.frame.height)) }
        }
        XCTAssertGreaterThan(text.filter { $0 > 120 }.count, 20, "The session note should show under Sign out")
    }

    @MainActor
    func testAudioOptionsButtonIsLegible() {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        launchAtSearch(app)
        for key in [XCUIRemote.Button.down, .down, .right, .right, .select] { remote.press(key) }
        XCTAssertTrue(app.staticTexts["Albums"].waitForExistence(timeout: 10))
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Tracks"].waitForExistence(timeout: 10))
        remote.press(.select)
        let options = app.buttons["Options"]
        XCTAssertTrue(options.waitForExistence(timeout: 20))
        settle()
        record("audio options", app)
        XCTAssertGreaterThan(iconHeight(app, in: options.frame), options.frame.height * 0.35, "The Options button should be large enough to read from the couch")
        for _ in 0..<4 where !options.hasFocus { remote.press(.up); settle() }
        XCTAssertTrue(options.hasFocus, "Up from the transport should reach Options: \(focusedLabels(app))")
        record("audio options focused", app)
        XCTAssertGreaterThan(iconHeight(app, in: options.frame), options.frame.height * 0.35, "The focused Options label should stay visible")
    }

    @MainActor
    private func iconHeight(_ app: XCUIApplication, in frame: CGRect) -> CGFloat {
        let image = app.screenshot().image
        let sample = { (x: CGFloat, y: CGFloat) in self.brightness(self.samplePixel(image, x: x / app.frame.width, y: y / app.frame.height)) }
        let base = sample(frame.midX, frame.minY + 6)
        let rows = stride(from: frame.minY + 4, through: frame.maxY - 4, by: 2).filter { y in
            stride(from: frame.minX + 4, through: frame.maxX - 4, by: 2).contains { abs(sample($0, y) - base) > 80 }
        }
        return (rows.last ?? 0) - (rows.first ?? 0)
    }

    @MainActor
    private func openShow() -> XCUIApplication {
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
        XCTAssertTrue(app.staticTexts["Episodes"].waitForExistence(timeout: 10))
        settle()
        record("show page", app)
        return app
    }

    private func settle() { RunLoop.current.run(until: Date().addingTimeInterval(1.5)) }
}
