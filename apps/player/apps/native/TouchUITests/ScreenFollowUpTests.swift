import XCTest

final class ScreenFollowUpTests: XCTestCase {
    private var server = ""
    private var token = ""

    @MainActor private func connectedApp() async throws -> XCUIApplication {
        let environment = ProcessInfo.processInfo.environment
        guard let server = environment["KINOSAIL_DESIGN_TOUR_SERVER"], !server.isEmpty else {
            throw XCTSkip("Set KINOSAIL_DESIGN_TOUR_SERVER to a populated test Server.")
        }
        self.server = server
        token = environment["KINOSAIL_DESIGN_TOUR_TOKEN"] ?? ""
        XCUIDevice.shared.orientation = .portrait
        let app = XCUIApplication(bundleIdentifier: "com.kinosail.player")
        app.launchArguments += ["-UIPreferredContentSizeCategoryName", "UICTContentSizeCategoryL"]
        app.launch()
        try await tourConnect(app, server: server, token: token, records: false)
        XCTAssertTrue(tourTab("Home", in: app).waitForExistence(timeout: 90), app.debugDescription)
        return app
    }

    private func call(_ path: String, method: String = "GET", body: String? = nil) async throws -> Data {
        var request = URLRequest(url: try XCTUnwrap(URL(string: server + path)))
        request.httpMethod = method
        request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        if let body {
            request.httpBody = Data(body.utf8)
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        let (data, response) = try await URLSession.shared.data(for: request)
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200, String(decoding: data, as: UTF8.self))
        return data
    }

    private func startEpisodeNamedLikeItsShow() async throws {
        let data = try await call("/api/v1/library?limit=200")
        let items = try XCTUnwrap((try JSONSerialization.jsonObject(with: data) as? [String: Any])?["items"] as? [[String: Any]])
        let episode = try XCTUnwrap(items.first { item in (item["show"] as? String).map { !$0.isEmpty && item["title"] as? String == $0 } ?? false },
                                    "The test Server needs an episode whose title is its show name")
        let opener = items.first { $0["show"] as? String == episode["show"] as? String && $0["season"] as? Int == 1 && $0["episode"] as? Int == 1 }
        for item in [opener, episode].compactMap({ $0 }) {
            let id = try XCTUnwrap(item["id"] as? String)
            _ = try await call("/api/v1/items/\(id)/progress", method: "PUT", body: #"{"seconds":12}"#)
        }
    }

    @MainActor private func scrollTo(_ element: XCUIElement, in app: XCUIApplication, swipes: Int = 10) {
        for _ in 0..<swipes where !(element.exists && element.isHittable) { app.swipeUp(velocity: .slow) }
    }

    @MainActor private func brightest(in frame: CGRect, _ app: XCUIApplication) -> (red: Int, green: Int, blue: Int) {
        let shot = XCUIScreen.main.screenshot().image
        guard let image = shot.cgImage else { return (0, 0, 0) }
        let scale = CGFloat(image.width) / app.windows.firstMatch.frame.width
        let area = CGRect(x: frame.minX * scale, y: frame.minY * scale, width: frame.width * scale, height: frame.height * scale).integral
        guard let crop = image.cropping(to: area) else { return (0, 0, 0) }
        let width = crop.width, height = crop.height
        var pixels = [UInt8](repeating: 0, count: width * height * 4)
        pixels.withUnsafeMutableBytes { bytes in
            let context = CGContext(data: bytes.baseAddress, width: width, height: height, bitsPerComponent: 8, bytesPerRow: width * 4,
                                    space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
            context?.draw(crop, in: CGRect(x: 0, y: 0, width: width, height: height))
        }
        var best = (red: 0, green: 0, blue: 0)
        for index in stride(from: 0, to: pixels.count, by: 4) {
            let color = (red: Int(pixels[index]), green: Int(pixels[index + 1]), blue: Int(pixels[index + 2]))
            if color.red + color.green + color.blue > best.red + best.green + best.blue { best = color }
        }
        return best
    }

    @MainActor func testEpisodeCardsNeverRepeatTheShowName() async throws {
        let app = try await connectedApp()
        try await startEpisodeNamedLikeItsShow()
        app.terminate()
        app.launch()
        XCTAssertTrue(tourTab("Home", in: app).waitForExistence(timeout: 60), app.debugDescription)
        let resume = app.buttons.matching(NSPredicate(format: "label CONTAINS 'S2 E1'")).firstMatch
        XCTAssertTrue(resume.waitForExistence(timeout: 30), app.debugDescription)
        scrollTo(resume, in: app)
        tourRecord("f1 episode cards", app)
        XCTAssertEqual(resume.label.components(separatedBy: "Example Show").count - 1, 1, "Show name repeats: \(resume.label)")
        XCTAssertTrue(resume.label.hasPrefix("Episode 1"), resume.label)
        let coded = app.descendants(matching: .any).matching(NSPredicate(format: "label BEGINSWITH 'S01E01' OR label CONTAINS ', S01E01'"))
        app.swipeDown()
        for _ in 0..<4 {
            XCTAssertEqual(coded.count, 0, "Titles repeat the episode code: \(coded.allElementsBoundByIndex.map(\.label))")
            app.swipeUp()
        }
    }

    @MainActor func testDetailLinksShowIconsAndGenresUseMutedText() async throws {
        let app = try await connectedApp()
        try await startEpisodeNamedLikeItsShow()
        app.terminate()
        app.launch()
        let resume = app.buttons.matching(NSPredicate(format: "label CONTAINS 'S2 E1'")).firstMatch
        XCTAssertTrue(resume.waitForExistence(timeout: 60), app.debugDescription)
        scrollTo(resume, in: app)
        resume.press(forDuration: 1.2)
        let details = app.buttons["Details"].firstMatch
        XCTAssertTrue(details.waitForExistence(timeout: 10), app.debugDescription)
        details.tap()
        XCTAssertTrue(app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Resume'")).firstMatch.waitForExistence(timeout: 20), app.debugDescription)
        for name in ["All episodes", "Remove from Continue watching"] {
            let link = app.buttons[name].firstMatch
            scrollTo(link, in: app)
            XCTAssertTrue(link.exists, app.debugDescription)
            XCTAssertTrue(link.images.firstMatch.exists, "\(name) has no icon")
        }
        tourRecord("f2 episode detail links", app)
        tourBack(app)
        tourReveal(tourTab("Movies", in: app), in: app)
        tourTab("Movies", in: app).tap()
        let movie = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Delta'")).firstMatch
        XCTAssertTrue(movie.waitForExistence(timeout: 20), app.debugDescription)
        scrollTo(movie, in: app)
        movie.tap()
        let genres = app.staticTexts["Drama"]
        XCTAssertTrue(genres.waitForExistence(timeout: 20), app.debugDescription)
        scrollTo(genres, in: app)
        sleep(1)
        tourRecord("f3 movie genres", app)
        let color = brightest(in: genres.frame, app)
        XCTAssertGreaterThan(color.green, color.blue, "Genres use a blue-gray system color, not the muted sage: \(color)")
    }

    @MainActor func testPrivacyPolicyRowAlignsWithOtherAboutRows() async throws {
        let app = try await connectedApp()
        tourTab("More", in: app).tap()
        let settings = app.buttons["Settings"].firstMatch
        XCTAssertTrue(settings.waitForExistence(timeout: 10), app.debugDescription)
        scrollTo(settings, in: app)
        settings.tap()
        let privacy = app.links["Privacy policy"].firstMatch
        let thanks = app.buttons["Made possible by"].firstMatch
        XCTAssertTrue(app.buttons["Change Server"].waitForExistence(timeout: 10), app.debugDescription)
        scrollTo(privacy, in: app)
        XCTAssertTrue(thanks.exists, app.debugDescription)
        tourRecord("f4 settings about", app)
        XCTAssertTrue(privacy.images.firstMatch.exists, "Privacy policy has no icon")
        let privacyText = privacy.staticTexts.firstMatch, thanksText = thanks.staticTexts.firstMatch
        if privacyText.exists && thanksText.exists {
            XCTAssertEqual(privacyText.frame.minX, thanksText.frame.minX, accuracy: 4, "About rows start at different positions")
        }
    }

    @MainActor func testDisabledReaderPageButtonLooksDimmerThanEnabled() async throws {
        let app = try await connectedApp()
        tourTab("More", in: app).tap()
        let books = app.buttons["Books"].firstMatch
        XCTAssertTrue(books.waitForExistence(timeout: 10), app.debugDescription)
        scrollTo(books, in: app)
        books.tap()
        let book = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Example Comic'")).firstMatch
        XCTAssertTrue(book.waitForExistence(timeout: 20), app.debugDescription)
        scrollTo(book, in: app)
        book.tap()
        let read = app.buttons.matching(NSPredicate(format: "label BEGINSWITH 'Read'")).firstMatch
        XCTAssertTrue(read.waitForExistence(timeout: 20), app.debugDescription)
        read.tap()
        let previous = app.buttons["Previous"], next = app.buttons["Next"]
        XCTAssertTrue(next.waitForExistence(timeout: 30), app.debugDescription)
        let deadline = Date().addingTimeInterval(20)
        while !(next.isEnabled && !previous.isEnabled) && Date() < deadline { sleep(1) }
        XCTAssertFalse(previous.isEnabled, app.debugDescription)
        XCTAssertTrue(next.isEnabled, app.debugDescription)
        sleep(1)
        tourRecord("f5 reader page buttons", app)
        let disabled = brightest(in: previous.frame, app), enabled = brightest(in: next.frame, app)
        XCTAssertLessThan(disabled.red + disabled.green + disabled.blue, enabled.red + enabled.green + enabled.blue,
                          "Disabled Previous \(disabled) is not dimmer than enabled Next \(enabled)")
    }
}
