import UIKit
import XCTest

final class RemoteArtworkControlRouteTests: RemoteTestCase {
    override func setUpWithError() throws {
        try super.setUpWithError()
        let environment = ProcessInfo.processInfo.environment
        try XCTSkipUnless(environment["KINOSAIL_TV_FIXTURE_QA"] == "1"
                          && environment["KINOSAIL_TV_QUALITY_FIXTURE_MODE"] == "loaded"
                          && environment["KINOSAIL_TV_ARTWORK_CAST_FIXTURE"] == "1",
                          "Requires the disposable loaded artwork cast fixture.")
    }

    @MainActor
    func testFirstDownRouteFromResumeIsRecorded() throws {
        continueAfterFailure = false
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        let remote = XCUIRemote.shared
        app.launch()
        XCTAssertTrue(app.buttons["Search"].waitForExistence(timeout: 15))
        remote.press(.down)
        remote.press(.down)
        XCTAssertTrue(app.buttons["Movies"].hasFocus)
        remote.press(.select)
        XCTAssertTrue(app.staticTexts["Sort: Title"].waitForExistence(timeout: 10))
        remote.press(.down)
        remote.press(.select)
        let title = app.staticTexts["After the Rain"]
        XCTAssertTrue(title.waitForExistence(timeout: 10))
        var previous = ""
        var stable = 0
        let loaded = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let current = self.signature(app)
            stable = current == previous ? stable + 1 : 0
            previous = current
            return stable >= 2 && app.buttons["Resume"].hasFocus
                && (try? self.hasLoadedLandscape(app)) == true
        }, object: nil)
        let loadedResult = XCTWaiter.wait(for: [loaded], timeout: 10)
        record("route baseline controls before first Down", app)
        let initial = try snapshot(app)
        XCTAssertEqual(loadedResult, .completed, "Require loaded pixels and stable initial controls")
        XCTAssertEqual(title.frame.minX, 184, accuracy: 2)
        XCTAssertTrue(app.buttons["Resume"].hasFocus)
        let initialResumeFrame = app.buttons["Resume"].frame

        remote.press(.down)
        record("route first Down before settling", app)
        let immediate = try snapshot(app)
        previous = ""
        stable = 0
        var samples: [[String: Any]] = []
        let settled = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let current = self.signature(app)
            stable = current == previous ? stable + 1 : 0
            previous = current
            if let sample = try? self.snapshot(app) { samples.append(sample) }
            return stable >= 2
        }, object: nil)
        let settledResult = XCTWaiter.wait(for: [settled], timeout: 8)
        record("route first Down settled before assertions", app)
        let final = try snapshot(app)
        let focused = controls(app).filter { $0.1.exists && $0.1.hasFocus }
        let report: [String: Any] = [
            "initial": initial, "immediate": immediate, "samples": samples,
            "final": final, "settled": settledResult == .completed,
            "firstDownDestination": focused.count == 1 ? focused[0].0 : "",
            "focusedControlCount": focused.count
        ]
        let data = try JSONSerialization.data(withJSONObject: report, options: [.sortedKeys])
        print("ARTWORK_CONTROL_ROUTE_JSON \(String(decoding: data, as: UTF8.self))")
        let attachment = XCTAttachment(data: data, uniformTypeIdentifier: "public.json")
        attachment.name = "movie first Down route observations"
        attachment.lifetime = .keepAlways
        add(attachment)
        XCTAssertEqual(settledResult, .completed, "Do not compare an unsettled focus transition")
        XCTAssertEqual(focused.count, 1, "Require one actual settled action focus")
        XCTAssertNotEqual(focused.first?.0, "Resume", "Down must leave the primary action")
        let destination = try XCTUnwrap(focused.first?.1)
        XCTAssertTrue(destination.isHittable)
        XCTAssertGreaterThanOrEqual(destination.frame.minX, app.frame.minX + 78)
        XCTAssertLessThanOrEqual(destination.frame.maxX, app.frame.maxX - 78)
        XCTAssertGreaterThanOrEqual(destination.frame.minY, app.frame.minY + 58)
        XCTAssertLessThanOrEqual(destination.frame.maxY, app.frame.maxY - 58)

        let myList = app.buttons["My List"]
        for attempt in 1...2 where !myList.hasFocus {
            remote.press(.left)
            try observeSettledControls("Left \(attempt) toward My List", app)
        }
        try observeSettledControls("My List reachability", app)
        XCTAssertTrue(myList.hasFocus, "My List must remain reachable within this three-action row")
        XCTAssertTrue(myList.isHittable)
        remote.press(.right)
        try observeSettledControls("Right from My List to Bookmarks", app)
        XCTAssertTrue(app.buttons["Bookmarks"].hasFocus)
        XCTAssertTrue(app.buttons["Bookmarks"].isHittable)
        remote.press(.up)
        try observeSettledControls("Up from Bookmarks to Resume", app)
        XCTAssertTrue(app.buttons["Resume"].hasFocus)
        XCTAssertTrue(app.buttons["Resume"].isHittable)
        XCTAssertEqual(app.buttons["Resume"].frame.minX, initialResumeFrame.minX, accuracy: 2)
        XCTAssertEqual(app.buttons["Resume"].frame.minY, initialResumeFrame.minY, accuracy: 2)
    }

    @MainActor private func observeSettledControls(_ stage: String, _ app: XCUIApplication) throws {
        var previous = ""
        var stable = 0
        let settled = XCTNSPredicateExpectation(predicate: NSPredicate { _, _ in
            let current = self.signature(app)
            stable = current == previous ? stable + 1 : 0
            previous = current
            return stable >= 2
        }, object: nil)
        let result = XCTWaiter.wait(for: [settled], timeout: 5)
        record("route reachability \(stage) before assertions", app)
        let data = try JSONSerialization.data(withJSONObject:
            ["stage": stage, "settled": result == .completed, "observation": snapshot(app)],
            options: [.sortedKeys])
        print("ARTWORK_CONTROL_REACHABILITY_SAMPLE_JSON \(String(decoding: data, as: UTF8.self))")
        XCTAssertEqual(result, .completed, "Require settled controls at \(stage)")
    }

    @MainActor private func controls(_ app: XCUIApplication) -> [(String, XCUIElement)] {
        ["Resume", "My List", "Bookmarks", "Remove from Continue watching"].map { ($0, app.buttons[$0]) }
    }

    @MainActor private func signature(_ app: XCUIApplication) -> String {
        controls(app).map { name, element in
            "\(name):\(element.exists):\(element.frame):\(element.hasFocus)"
        }.joined(separator: "|") + "|\(app.staticTexts["After the Rain"].frame)"
    }

    @MainActor private func snapshot(_ app: XCUIApplication) throws -> [String: Any] {
        let actions = controls(app).map { name, element -> [String: Any] in
            ["name": name, "identifier": element.identifier, "label": element.label,
             "frame": NSCoder.string(for: element.frame), "hasFocus": element.hasFocus,
             "exists": element.exists, "isHittable": element.isHittable]
        }
        return ["actions": actions, "titleFrame": NSCoder.string(for: app.staticTexts["After the Rain"].frame),
                "focusedLabels": focusedLabels(app)]
    }

    @MainActor private func hasLoadedLandscape(_ app: XCUIApplication) throws -> Bool {
        let cg = try XCTUnwrap(app.screenshot().image.cgImage)
        let pixel = try XCTUnwrap(cg.cropping(to: CGRect(x: 1728, y: 270, width: 1, height: 1)))
        let context = try XCTUnwrap(CGContext(data: nil, width: 1, height: 1,
            bitsPerComponent: 8, bytesPerRow: 4, space: CGColorSpaceCreateDeviceRGB(),
            bitmapInfo: CGBitmapInfo.byteOrder32Big.rawValue | CGImageAlphaInfo.premultipliedLast.rawValue))
        context.draw(pixel, in: CGRect(x: 0, y: 0, width: 1, height: 1))
        let bytes = try XCTUnwrap(context.data?.assumingMemoryBound(to: UInt8.self))
        return Int(bytes[2]) > Int(bytes[1]) + 25 && Int(bytes[2]) > Int(bytes[0]) + 25
    }
}
