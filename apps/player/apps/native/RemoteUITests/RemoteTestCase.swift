import XCTest

class RemoteTestCase: XCTestCase {
    @MainActor
    func focusedLabels(_ app: XCUIApplication) -> [String] {
        app.descendants(matching: .any).matching(NSPredicate(format: "hasFocus == true"))
            .allElementsBoundByIndex.flatMap { element in
                // Native Menu focus belongs to a container whose visible
                // title is exposed through its static-text children.
                element.label.isEmpty ? element.staticTexts.allElementsBoundByIndex.map(\.label)
                                      : [element.label]
            }
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
