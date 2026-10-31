import XCTest

extension XCTestCase {
    @MainActor func openActor(_ app: XCUIApplication, from movie: String, record: String) throws {
        let title = { app.buttons.matching(NSPredicate(format: "label BEGINSWITH %@", movie)).firstMatch }
        title().tap()
        XCTAssertTrue(app.buttons["Play"].firstMatch.waitForExistence(timeout: 30), app.debugDescription)
        let cast = app.staticTexts["Cast"]
        for _ in 0..<5 where !(cast.exists && cast.isHittable) { app.swipeUp() }
        XCTAssertTrue(cast.exists, "\(movie) should list its cast; \(app.debugDescription)")
        let person = try XCTUnwrap(app.buttons.allElementsBoundByIndex.first { $0.frame.minY > cast.frame.maxY && $0.isHittable }, app.debugDescription)
        person.tap()
        XCTAssertTrue(title().waitForExistence(timeout: 30), "The actor page should list \(movie); \(app.debugDescription)")
        sleep(2)
        tourRecord(record, app)
        app.navigationBars.buttons.firstMatch.tap()
        app.navigationBars.buttons.firstMatch.tap()
    }
}
