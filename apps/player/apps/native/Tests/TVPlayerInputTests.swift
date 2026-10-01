#if os(tvOS)
import AVKit
import XCTest
@testable import KinosailPlayer

final class TVPlayerInputTests: XCTestCase {
    @MainActor func testEmbeddedPlayerDoesNotDismissItsHostDuringAVKitInput() {
        let presentation = PlayerPresentation()
        let allowed = presentation.controller.delegate?.playerViewControllerShouldDismiss?(presentation.controller) ?? true
        XCTAssertFalse(allowed, "The embedded player must leave dismissal to its owner after remote input returns")
    }

    @MainActor func testBackDefersAndCoalescesTheOwnersDismissal() async throws {
        let presentation = PlayerPresentation()
        var closed = 0
        presentation.close = { closed += 1 }
        for _ in 0..<2 {
            presentation.requestClose()
        }
        XCTAssertEqual(closed, 0, "Remote input must return before the player view is removed")
        try await Task.sleep(for: .milliseconds(50))
        XCTAssertEqual(closed, 1)
    }

    @MainActor func testForeignAndClearedPlayersCannotCloseTheOwner() async throws {
        let presentation = PlayerPresentation()
        var closed = 0
        presentation.close = { closed += 1 }
        XCTAssertFalse(presentation.controller.delegate?.playerViewControllerShouldDismiss?(AVPlayerViewController()) ?? true)
        try await Task.sleep(for: .milliseconds(50))
        XCTAssertEqual(closed, 0)

        presentation.requestClose()
        presentation.clear()
        presentation.close = { closed += 1 }
        try await Task.sleep(for: .milliseconds(50))
        XCTAssertEqual(closed, 0, "Session cleanup must cancel an already queued dismissal")
    }
}
#endif
