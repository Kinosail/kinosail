#if os(iOS)
import Foundation
import Testing
@testable import KinosailPlayer

struct LibraryIntentTests {
    @Test(arguments: ["", " ", "a\nb", String(repeating: "x", count: 513)])
    @MainActor func invalidSearchDoesNotRestoreOrOpenSession(value: String) async {
        let session = AppSession()
        await #expect(throws: (any Error).self) {
            try await IntentLibrary.open(action: .search, value: value, session: session)
        }
        #expect(session.restoring)
        #expect(session.client == nil)
        #expect(session.pendingMediaLink == nil)
        #expect(session.player.currentItem == nil)
    }

    @Test(arguments: ["", " ", "title\t", String(repeating: "x", count: 513)])
    @MainActor func invalidPlayDoesNotRestoreOrStartPlayback(value: String) async {
        let session = AppSession()
        await #expect(throws: (any Error).self) {
            try await IntentLibrary.open(action: .play, value: value, session: session)
        }
        #expect(session.restoring)
        #expect(session.client == nil)
        #expect(session.pendingMediaLink == nil)
        #expect(session.player.currentItem == nil)
    }
}
#endif
