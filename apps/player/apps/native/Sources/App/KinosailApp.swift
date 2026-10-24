import SwiftUI
import UIKit
#if os(iOS)
import AppIntents
#endif

@main
struct KinosailApp: App {
    #if os(iOS)
    @UIApplicationDelegateAdaptor(AppleAppDelegate.self) private var appDelegate
    #endif
    @State private var session: AppSession

    init() {
        let session = AppSession()
        _session = State(initialValue: session)
        #if os(iOS)
        AppDependencyManager.shared.add(dependency: session)
        #endif
        #if os(tvOS)
        RemoteTouchDelays.install()
        let bar = UINavigationBar.appearance()
        bar.titleTextAttributes = [.foregroundColor: UIColor(KinoTheme.text).resolvedColor(with: UITraitCollection(userInterfaceStyle: .dark))]
        bar.isTranslucent = true
        bar.tintColor = UIColor(KinoTheme.signal).resolvedColor(with: UITraitCollection(userInterfaceStyle: .dark))
        UITabBar.appearance().tintColor = UIColor(KinoTheme.signalInk).resolvedColor(with: UITraitCollection(userInterfaceStyle: .dark))
        #endif
    }

    var body: some Scene {
        WindowGroup {
            LaunchingAppShell()
                .tint(KinoTheme.signal)
                #if os(tvOS)
                .modifier(TopShelfPublishing())
                .preferredColorScheme(.dark)
                .foregroundStyle(KinoTheme.text, KinoTheme.muted)
                #endif
                .environment(session)
                .onOpenURL { url in Task { await session.restore(); session.handleIncomingURL(url) } }
        }
    }
}

private struct LaunchingAppShell: View {
    @Environment(AppSession.self) private var session
    @State private var launchMinimumElapsed = false

    var body: some View {
        LaunchReveal(ready: launchMinimumElapsed && !session.restoring) { AppShell() }
            .task {
                do { try await Task.sleep(for: .milliseconds(120)) }
                catch { return }
                launchMinimumElapsed = true
            }
    }
}
