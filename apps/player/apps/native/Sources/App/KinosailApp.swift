import SwiftUI
import UIKit

@main
struct KinosailApp: App {
    #if os(iOS)
    @UIApplicationDelegateAdaptor(AppleAppDelegate.self) private var appDelegate
    #endif
    @State private var session = AppSession()

    init() {
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
                .environment(session)
                .tint(KinoTheme.signal)
                #if os(tvOS)
                .preferredColorScheme(.dark)
                .foregroundStyle(KinoTheme.text, KinoTheme.muted)
                #endif
                .onOpenURL { session.handleIncomingURL($0) }
        }
    }
}
