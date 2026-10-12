#if os(tvOS) && targetEnvironment(simulator)
import SwiftUI
import UIKit
import XCTest
@testable import KinosailPlayer

// Isolated view test: public traits on a temporary host, not system Settings.
@MainActor final class TVDetailLargeTextHostTests: XCTestCase {
    func testAccessibilityXXXLDetailActionsAlignAndReceiveNativeFocus() async throws {
        try XCTSkipUnless(ProcessInfo.processInfo.environment["KINOSAIL_TV_LARGE_TEXT_HOST"] == "1",
                          "Requires the disposable loopback fixture on an explicitly owned simulator")
        let environment = ProcessInfo.processInfo.environment
        guard environment["KINOSAIL_TV_QUALITY_FIXTURE_MODE"] == "loaded",
              let identity = environment["KINOSAIL_TV_QUALITY_FIXTURE_ID"],
              identity.range(of: "^tv-polish-qa-[a-z0-9-]{1,64}$", options: .regularExpression) != nil else {
            throw ClientError.invalidInput("The isolated host requires a fresh loaded fixture identity.")
        }
        try await TVQualitySessionSeed().seedDisposableSimulatorSession()
        let session = AppSession()
        await session.restore()
        let client = try XCTUnwrap(session.client)
        do {
            let server = await client.server
            guard server == (try ServerAddress("http://127.0.0.1:38359")),
                  session.viewer?.id == "qa", session.viewer?.serverID == identity else {
                throw ClientError.invalidResponse
            }
            try await render(session: session)
        } catch {
            session.player.stop()
            await client.close()
            throw error
        }
        session.player.stop()
        await client.close()
        print("LARGE_TEXT_TRAIT_HOST clientClosed=true")
    }

    private func render(session: AppSession) async throws {
        let scene = try XCTUnwrap(UIApplication.shared.connectedScenes.first as? UIWindowScene)
        let previous = try XCTUnwrap(scene.windows.first { $0.isKeyWindow })
        let window = UIWindow(windowScene: scene)
        let probe = TypeReadback()
        let host = UIHostingController(rootView: NavigationStack {
            DetailScreen(itemID: "movie-004").background(TypeProbe(probe: probe))
        }.environment(session).environment(\.dynamicTypeSize, .accessibility5)
            .tint(KinoTheme.signal).preferredColorScheme(.dark))
        host.traitOverrides.preferredContentSizeCategory = .accessibilityExtraExtraExtraLarge
        window.rootViewController = host
        window.makeKeyAndVisible()
        defer {
            window.isHidden = true
            window.rootViewController = nil
            previous.makeKey()
            XCTAssertTrue(previous.isKeyWindow, "Restore the existing app window")
            XCTAssertTrue(window.isHidden && window.rootViewController == nil)
            print("LARGE_TEXT_TRAIT_HOST temporaryHostRemoved=true originalWindowRestored=\(previous.isKeyWindow)")
        }
        do {
            try await until(deadline: ContinuousClock.now.advanced(by: .seconds(10))) { self.item("Resume", in: window) != nil }
            XCTAssertEqual(probe.value, .accessibility5)
            XCTAssertEqual(host.traitCollection.preferredContentSizeCategory, .accessibilityExtraExtraExtraLarge)
            print("LARGE_TEXT_TRAIT_HOST SwiftUI=\(String(describing: probe.value)) UIKit=\(host.traitCollection.preferredContentSizeCategory.rawValue)")
            let system = try XCTUnwrap(scene.focusSystem)
            for label in ["Resume", "My List", "Bookmarks", "Remove from Continue watching", "Resume"] {
                let target = try XCTUnwrap(item(label, in: window), "Actual native focus item for \(label)")
                let focusEnd = ContinuousClock.now.advanced(by: .seconds(5))
                system.requestFocusUpdate(to: target)
                system.updateFocusIfNeeded()
                try await until(deadline: focusEnd) { system.focusedItem === target }
                guard ContinuousClock.now.advanced(by: .milliseconds(300)) < focusEnd else { throw ClientError.unavailable }
                try await Task.sleep(for: .milliseconds(300))
                guard ContinuousClock.now < focusEnd, system.focusedItem === target else { throw ClientError.unavailable }
                window.layoutIfNeeded()
                let frame = try bounds(target, in: window)
                XCTAssertTrue(window.bounds.contains(frame), "Focused \(label) must be fully inside the visible host: \(frame)")
                try record(label, window: window)
                if label == "Bookmarks" {
                    let primary = try bounds(XCTUnwrap(item("Resume", in: window)), in: window)
                    let list = try bounds(XCTUnwrap(item("My List", in: window)), in: window)
                    let remove = try bounds(XCTUnwrap(item("Remove from Continue watching", in: window)), in: window)
                    XCTAssertEqual(primary.height, list.height, accuracy: 1)
                    XCTAssertEqual(primary.height, remove.height, accuracy: 1)
                    XCTAssertEqual(primary.minX, list.minX, accuracy: 1)
                    XCTAssertEqual(primary.minX, remove.minX, accuracy: 1)
                }
                print("LARGE_TEXT_TRAIT_HOST category=accessibility5 focused=\(label) bounds=\(frame)")
            }
        } catch {
            try? record("prerequisite-or-assertion-failure", window: window)
            throw error
        }
    }

    private func item(_ label: String, in window: UIWindow) -> (any UIFocusItem)? {
        var pending: [any UIFocusEnvironment] = [window]
        var seen = Set<ObjectIdentifier>()
        while let environment = pending.popLast(), seen.count < 1024 {
            guard seen.insert(ObjectIdentifier(environment)).inserted else { continue }
            if let item = environment as? any UIFocusItem, item.canBecomeFocused,
               let object = item as? NSObject, object.accessibilityLabel == label { return item }
            if let container = environment.focusItemContainer {
                let search = window.bounds.insetBy(dx: -window.bounds.width, dy: -window.bounds.height * 4)
                let rect = container.coordinateSpace.convert(search, from: window)
                pending.append(contentsOf: container.focusItems(in: rect))
            }
        }
        return nil
    }

    private func bounds(_ item: any UIFocusItem, in window: UIWindow) throws -> CGRect {
        let frame: CGRect
        if let view = item as? UIView { frame = view.convert(view.bounds, to: window) }
        else {
            let container = try XCTUnwrap(item.parentFocusEnvironment?.focusItemContainer)
            frame = window.convert(item.frame, from: container.coordinateSpace)
        }
        guard !frame.isNull, frame.width > 0, frame.height > 0,
              [frame.minX, frame.minY, frame.maxX, frame.maxY].allSatisfy({ $0.isFinite }) else {
            throw ClientError.invalidResponse
        }
        return frame
    }

    private func until(deadline end: ContinuousClock.Instant, _ ready: () -> Bool) async throws {
        while !ready(), ContinuousClock.now < end { try await Task.sleep(for: .milliseconds(50)) }
        let accepted = ContinuousClock.now < end && ready()
        XCTAssertTrue(accepted, "Existing loaded10/focus5 deadline")
        if !accepted { throw ClientError.unavailable }
    }

    private func record(_ label: String, window: UIWindow) throws {
        let format = UIGraphicsImageRendererFormat()
        format.scale = 1
        var painted = false
        let image = UIGraphicsImageRenderer(bounds: window.bounds, format: format).image { _ in
            painted = window.drawHierarchy(in: window.bounds, afterScreenUpdates: true)
        }
        guard painted, image.size == window.bounds.size, image.cgImage != nil else { throw ClientError.invalidResponse }
        let attachment = XCTAttachment(image: image)
        attachment.name = "detail-large-text-trait-accessibility5-\(label)-\(UUID().uuidString)"
        attachment.lifetime = .keepAlways
        add(attachment)
    }
}

@MainActor private final class TypeReadback { var value: DynamicTypeSize? }
private struct TypeProbe: View {
    let probe: TypeReadback
    @Environment(\.dynamicTypeSize) private var size
    var body: some View { Color.clear.onAppear { probe.value = size } }
}
#endif
