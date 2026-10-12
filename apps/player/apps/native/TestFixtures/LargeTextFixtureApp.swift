#if os(tvOS) && targetEnvironment(simulator)
import Observation
import SwiftUI
import UIKit

// Disposable, opt-in app entry point. Only the temporary fixture project
// compiles this file; the production app and its entry point are unchanged.
@main @MainActor final class LargeTextFixtureApp: UIResponder, UIApplicationDelegate {
    var window: UIWindow?

    func application(_ application: UIApplication,
                     didFinishLaunchingWithOptions options: [UIApplication.LaunchOptionsKey: Any]? = nil) -> Bool {
        let env = ProcessInfo.processInfo.environment
        guard Bundle.main.bundleIdentifier == "com.kinosail.player.large-text-fixture",
              env["KINOSAIL_TV_LARGE_TEXT_FIXTURE"] == "1",
              env["KINOSAIL_TV_QUALITY_FIXTURE_MODE"] == "loaded",
              let identity = env["KINOSAIL_TV_QUALITY_FIXTURE_ID"],
              identity.range(of: "^tv-polish-qa-[a-z0-9-]{1,64}$", options: .regularExpression) != nil else {
            fatalError("Requires the disposable loaded fixture and its fresh identity")
        }
        let window = UIWindow(frame: UIScreen.main.bounds)
        window.rootViewController = UIViewController()
        window.backgroundColor = .black
        window.makeKeyAndVisible()
        self.window = window
        Task {
            do {
                let session = try await fixtureSession(identity: identity)
                let readback = LargeTextReadback(identity: identity)
                let host = UIHostingController(rootView: LargeTextFixtureRoot(session: session, readback: readback))
                host.traitOverrides.preferredContentSizeCategory = .accessibilityExtraExtraExtraLarge
                readback.host = host
                window.rootViewController = host
                window.makeKeyAndVisible()
                readback.updateUIKit()
            } catch {
                window.rootViewController = UIHostingController(rootView: Text("Fixture bootstrap failed"))
            }
        }
        return true
    }

    private func fixtureSession(identity: String) async throws -> AppSession {
        let server = try ServerAddress("http://127.0.0.1:38359")
        let client = try ServerClient(server: server, token: "tv-polish-fixture-token")
        do {
            let viewer = try await client.viewer()
            guard viewer.id == "qa", viewer.serverID == identity else { throw ClientError.invalidResponse }
            // This distinct fixture bundle has no shared Keychain entitlement.
            // Save only the known synthetic session; never read an existing credential.
            try await SessionKeychain().save(SavedSession(server: server, token: "tv-polish-fixture-token", viewer: viewer))
        } catch {
            await client.close()
            throw error
        }
        await client.close()
        let session = AppSession()
        await session.restore()
        guard let restored = session.client else { throw ClientError.invalidResponse }
        let restoredServer = await restored.server
        guard restoredServer == server, session.viewer?.id == "qa", session.viewer?.serverID == identity else {
            await restored.close()
            throw ClientError.invalidResponse
        }
        return session
    }
}

@MainActor @Observable private final class LargeTextReadback {
    let identity: String
    var root: DynamicTypeSize?
    var detail: DynamicTypeSize?
    var uiKit = "pending"
    @ObservationIgnored weak var host: UIViewController?
    init(identity: String) { self.identity = identity }
    func updateUIKit() {
        uiKit = host?.traitCollection.preferredContentSizeCategory.rawValue ?? "pending"
    }
    var label: String {
        "\(identity) root=\(root.map { String(describing: $0) } ?? "pending") detail=\(detail.map { String(describing: $0) } ?? "pending") UIKit=\(uiKit)"
    }
}

private struct LargeTextFixtureRoot: View {
    let session: AppSession
    let readback: LargeTextReadback
    @State private var path: [ScreenDestination] = [.detail("movie-004")]

    var body: some View {
        NavigationStack(path: $path) {
            LibraryScreen(initialView: .movies)
                .navigationDestination(for: ScreenDestination.self) { destination in
                    if case .detail(let id) = destination {
                        DetailScreen(itemID: id)
                            .background(LargeTextProbe(readback: readback, isDetail: true))
                            .dynamicTypeSize(.accessibility5)
                    } else { DestinationScreen(destination: destination) }
                }
        }
        .background(LargeTextProbe(readback: readback, isDetail: false))
        .environment(session)
        .dynamicTypeSize(.accessibility5)
        .tint(KinoTheme.signal)
        .preferredColorScheme(.dark)
        .foregroundStyle(KinoTheme.text, KinoTheme.muted)
        .overlay(alignment: .topTrailing) {
            Text(readback.label).font(.system(size: 12)).padding(4)
                .background(.black.opacity(0.8))
                .accessibilityIdentifier("fixture.large-text.readback")
                .allowsHitTesting(false)
        }
    }
}

private struct LargeTextProbe: View {
    let readback: LargeTextReadback
    let isDetail: Bool
    @Environment(\.dynamicTypeSize) private var size
    var body: some View {
        Color.clear.onChange(of: size, initial: true) { _, value in
            if isDetail { readback.detail = value } else { readback.root = value }
            readback.updateUIKit()
            print("LARGE_TEXT_FIXTURE \(isDetail ? "detail" : "root")=\(value) UIKit=\(readback.uiKit)")
        }
    }
}
#endif
