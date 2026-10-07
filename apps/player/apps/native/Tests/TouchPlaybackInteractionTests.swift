#if os(iOS)
import SwiftUI
import XCTest
@testable import KinosailPlayer

@MainActor final class TouchPlaybackInteractionTests: XCTestCase {
    func testOpeningVideoShowsProgressInsteadOfPlayAndKeepsCloseAvailable() async throws {
        for (name, size, vertical, type) in [
            ("phone", CGSize(width: 393, height: 852), UserInterfaceSizeClass.regular, DynamicTypeSize.large),
            ("landscape", CGSize(width: 852, height: 393), .compact, .accessibility1),
            ("tablet", CGSize(width: 1024, height: 1366), .regular, .large),
        ] {
            let labels = try await capture(failure: nil, name: name, size: size, vertical: vertical, type: type)
            XCTAssertTrue(labels.contains("Opening video…"), "Accessibility labels: \(labels)")
            XCTAssertTrue(labels.contains("Close player"))
            XCTAssertFalse(labels.contains("Play"), "A video that is still opening must not offer Play")
        }
    }

    func testFailedVideoRemovesLoadingAndKeepsRetryAndCloseAvailable() async throws {
        let labels = try await capture(failure: "The video could not be opened.", name: "failed",
                                       size: CGSize(width: 393, height: 852), vertical: .regular, type: .large)
        XCTAssertTrue(labels.contains("Try again"))
        XCTAssertTrue(labels.contains("Close player"))
        XCTAssertFalse(labels.contains("Opening video…"))
        XCTAssertFalse(labels.contains("Play"))
    }

    private func capture(failure: String?, name: String, size: CGSize,
                         vertical: UserInterfaceSizeClass, type: DynamicTypeSize) async throws -> [String] {
        let session = AppSession()
        let host = UIHostingController(rootView: TouchPlaybackView(failure: failure, retry: {}, close: {}).environment(session)
            .environment(\.verticalSizeClass, vertical).environment(\.dynamicTypeSize, type))
        let window = UIWindow(frame: CGRect(origin: .zero, size: size))
        window.windowScene = UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first
        window.frame = CGRect(origin: .zero, size: size)
        window.rootViewController = host
        window.makeKeyAndVisible()
        defer { window.isHidden = true }
        try await Task.sleep(for: .milliseconds(300))
        host.view.layoutIfNeeded()
        let attachment = XCTAttachment(image: UIGraphicsImageRenderer(bounds: host.view.bounds).image { _ in
            host.view.drawHierarchy(in: host.view.bounds, afterScreenUpdates: true)
        })
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
        return accessibilityLabels(host.view)
    }

    private func accessibilityLabels(_ view: UIView) -> [String] {
        var labels: [String] = []
        var visited = Set<ObjectIdentifier>()
        func visit(_ object: NSObject) {
            guard visited.insert(ObjectIdentifier(object)).inserted, visited.count < 1000 else { return }
            if let label = object.accessibilityLabel { labels.append(label) }
            if let children = object.accessibilityElements {
                for case let child as NSObject in children { visit(child) }
            } else {
                let count = object.accessibilityElementCount()
                if count > 0 && count < 1000 {
                    for index in 0..<count {
                        if let child = object.accessibilityElement(at: index) as? NSObject { visit(child) }
                    }
                }
            }
            if let view = object as? UIView { for child in view.subviews { visit(child) } }
        }
        visit(view)
        return labels
    }
}
#endif
