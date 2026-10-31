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
    func holdsFocus(_ element: XCUIElement) -> Bool {
        element.hasFocus || element.descendants(matching: .any).matching(NSPredicate(format: "hasFocus == true")).count > 0
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

    @MainActor
    func lightShare(_ app: XCUIApplication, in frame: CGRect) -> Double {
        let image = app.screenshot().image
        var light = 0, total = 0
        for x in stride(from: frame.minX, through: frame.maxX, by: 6) {
            for y in stride(from: frame.minY, through: frame.maxY, by: 6) {
                let color = samplePixel(image, x: x / app.frame.width, y: y / app.frame.height)
                total += 1
                if color.prefix(3).allSatisfy({ $0 > 220 }) { light += 1 }
            }
        }
        return total == 0 ? 0 : Double(light) / Double(total)
    }

    func brightness(_ color: [UInt8]) -> Int { color.prefix(3).map(Int.init).max() ?? 0 }

    func samplePixel(_ image: UIImage, x: CGFloat, y: CGFloat) -> [UInt8] {
        guard let cg = image.cgImage,
              let sample = cg.cropping(to: CGRect(x: CGFloat(cg.width) * x, y: CGFloat(cg.height) * y, width: 1, height: 1))
        else { return [0, 0, 0, 0] }
        var color = [UInt8](repeating: 0, count: 4)
        color.withUnsafeMutableBytes { bytes in
            let context = CGContext(data: bytes.baseAddress, width: 1, height: 1, bitsPerComponent: 8,
                                    bytesPerRow: 4, space: CGColorSpaceCreateDeviceRGB(),
                                    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
            context.draw(sample, in: CGRect(x: 0, y: 0, width: 1, height: 1))
        }
        return color
    }
}
