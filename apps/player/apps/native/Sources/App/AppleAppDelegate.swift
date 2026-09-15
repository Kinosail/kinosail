#if os(iOS)
import UIKit

@MainActor
final class AppleAppDelegate: NSObject, UIApplicationDelegate {
    func application(_ application: UIApplication, handleEventsForBackgroundURLSession identifier: String,
                     completionHandler: @escaping () -> Void) {
        guard identifier == VerifiedDownloads.identifier else { completionHandler(); return }
        let completion = BackgroundEventCompletion(completionHandler)
        let engine = VerifiedDownloads.shared
        engine.backgroundCompletion { completion.finish() }
        Task {
            do {
                if let saved = try await SessionKeychain().restore(), saved.viewer.downloads {
                    let access = try DownloadAuthorization(server: saved.server, serverID: saved.viewer.serverID, profileID: saved.viewer.id, token: saved.token)
                    try await engine.authorize(access)
                } else { await engine.lock() }
            } catch { await engine.lock(); completion.finish() }
        }
    }
}

@MainActor
private final class BackgroundEventCompletion {
    private var handler: (() -> Void)?
    init(_ handler: @escaping () -> Void) { self.handler = handler }
    func finish() { let handler = handler; self.handler = nil; handler?() }
}
#endif
