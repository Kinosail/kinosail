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
        // tvOS requires the legacy bar API even though the newer appearance
        // properties compile; setting standardAppearance raises at runtime.
        let bar = UINavigationBar.appearance()
        bar.titleTextAttributes = [.foregroundColor: UIColor(KinoTheme.text).resolvedColor(with: UITraitCollection(userInterfaceStyle: .dark))]
        bar.barTintColor = UIColor(KinoTheme.background).resolvedColor(with: UITraitCollection(userInterfaceStyle: .dark))
        bar.isTranslucent = false
        bar.tintColor = UIColor(KinoTheme.signal).resolvedColor(with: UITraitCollection(userInterfaceStyle: .dark))
        UITabBar.appearance().tintColor = UIColor(KinoTheme.signalInk).resolvedColor(with: UITraitCollection(userInterfaceStyle: .dark))
        #endif
    }

    var body: some Scene {
        WindowGroup {
            AppShell()
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
