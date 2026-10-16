import XCTest

class RemoteTestCase: XCTestCase {
    @MainActor
    func focusedLabels(_ app: XCUIApplication) -> [String] {
        app.descendants(matching: .any).matching(NSPredicate(format: "hasFocus == true"))
            .allElementsBoundByIndex.flatMap { element in
                element.label.isEmpty ? element.staticTexts.allElementsBoundByIndex.map(\.label)
                                      : [element.label]
            }
    }

    @MainActor
    func launchAtSearch(_ app: XCUIApplication) {
        app.launch()
        settleAtSearch(app)
    }

    @MainActor
    func settleAtSearch(_ app: XCUIApplication) {
        let search = app.buttons["Search"]
        XCTAssertTrue(search.waitForExistence(timeout: 15), app.debugDescription)
        let deadline = Date().addingTimeInterval(5)
        while !search.hasFocus && focusedLabels(app).isEmpty && Date() < deadline { RunLoop.current.run(until: Date().addingTimeInterval(0.2)) }
        if !search.hasFocus { XCUIRemote.shared.press(.up) }
        XCTAssertTrue(search.hasFocus, "Up from the first Home title should reach Search")
    }

    @MainActor
    func record(_ name: String, _ app: XCUIApplication) {
        print("REMOTE \(name): \(focusedLabels(app))")
        let attachment = XCTAttachment(screenshot: app.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }
}
