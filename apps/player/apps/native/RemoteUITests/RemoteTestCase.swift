import XCTest

class RemoteTestCase: XCTestCase {
    @MainActor
    func focusedLabels(_ app: XCUIApplication) -> [String] {
        app.descendants(matching: .any).matching(NSPredicate(format: "hasFocus == true"))
            .allElementsBoundByIndex.map(\.label)
    }

    @MainActor
    func record(_ name: String, _ app: XCUIApplication) {
        let focused = app.descendants(matching: .any).matching(
            NSPredicate(format: "hasFocus == true")
        ).allElementsBoundByIndex.map { "\($0.elementType):\($0.label)" }
        print("REMOTE \(name): \(focused)")
        let attachment = XCTAttachment(screenshot: app.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }
}
