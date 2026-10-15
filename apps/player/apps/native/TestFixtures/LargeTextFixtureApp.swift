#if os(tvOS) && targetEnvironment(simulator)
import Observation
import SwiftUI
import UIKit

@main @MainActor struct LargeTextFixtureApp: App {
    private let identity: String
    @State private var session: AppSession?
    @State private var readback: LargeTextReadback
    @State private var bootstrapFailed = false

    init() {
        let env = ProcessInfo.processInfo.environment
        guard Bundle.main.bundleIdentifier == "com.kinosail.player.large-text-fixture",
              env["KINOSAIL_TV_LARGE_TEXT_FIXTURE"] == "1",
              env["KINOSAIL_TV_QUALITY_FIXTURE_MODE"] == "loaded",
              let identity = env["KINOSAIL_TV_QUALITY_FIXTURE_ID"],
              identity.range(of: "^tv-polish-qa-[a-z0-9-]{1,64}$", options: .regularExpression) != nil else {
            fatalError("Requires the disposable loaded fixture and its fresh identity")
        }
        self.identity = identity
        _readback = State(initialValue: LargeTextReadback(identity: identity))
    }

    var body: some Scene {
        WindowGroup {
            Group {
                if let session {
                    LargeTextFixtureHost(session: session, readback: readback).ignoresSafeArea()
                } else {
                    Text(bootstrapFailed ? "Fixture bootstrap failed" : "Opening disposable fixture")
                }
            }
            .preferredColorScheme(.dark)
            .task {
                guard session == nil, !bootstrapFailed else { return }
                do { session = try await fixtureSession(identity: identity) }
                catch {
                    bootstrapFailed = true
                    print("LARGE_TEXT_FIXTURE bootstrap=failed errorType=\(String(describing: type(of: error)))")
                }
            }
        }
    }

    private func fixtureSession(identity: String) async throws -> AppSession {
        let server = try ServerAddress("http://127.0.0.1:38359")
        let client = try ServerClient(server: server, token: "tv-polish-fixture-token")
        do {
            let viewer = try await client.viewer()
            try Task.checkCancellation()
            guard viewer.id == "qa", viewer.serverID == identity else { throw ClientError.invalidResponse }
            try await SessionKeychain().save(SavedSession(server: server, token: "tv-polish-fixture-token", viewer: viewer))
        } catch {
            await client.close()
            throw error
        }
        await client.close()
        try Task.checkCancellation()
        let session = AppSession()
        await session.restore()
        guard let restored = session.client else { throw ClientError.invalidResponse }
        do {
            try Task.checkCancellation()
            let restoredServer = await restored.server
            try Task.checkCancellation()
            guard !session.restoring, session.notice == nil, restoredServer == server,
                  session.viewer?.id == "qa", session.viewer?.serverID == identity else { throw ClientError.invalidResponse }
            return session
        } catch {
            session.player.stop()
            await restored.close()
            throw error
        }
    }
}

private struct LargeTextFixtureHost: UIViewControllerRepresentable {
    let session: AppSession
    let readback: LargeTextReadback

    func makeUIViewController(context: Context) -> LargeTextFixtureController {
        LargeTextFixtureController(session: session, readback: readback)
    }
    func updateUIViewController(_ controller: LargeTextFixtureController, context: Context) {}
}

@MainActor private final class LargeTextFixtureController: UIHostingController<LargeTextFixtureRoot> {
    private let readback: LargeTextReadback

    init(session: AppSession, readback: LargeTextReadback) {
        self.readback = readback
        super.init(rootView: LargeTextFixtureRoot(session: session, readback: readback))
        traitOverrides.preferredContentSizeCategory = .accessibilityExtraExtraExtraLarge
        readback.host = self
    }
    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError("Fixture hosts are created programmatically") }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        readback.updateUIKit()
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
        guard let host, host.viewIfLoaded?.window?.windowScene != nil else { uiKit = "pending"; return }
        uiKit = host.traitCollection.preferredContentSizeCategory.rawValue
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
