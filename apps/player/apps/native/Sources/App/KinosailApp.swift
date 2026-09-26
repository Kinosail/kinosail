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
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var showingLaunch = true
    @State private var launchMinimumElapsed = false

    var body: some View {
        ZStack {
            AppShell()
                .accessibilityHidden(showingLaunch)
            ZStack {
                Color.black
                Image("LaunchMark")
                    .resizable()
                    .scaledToFit()
                    #if os(tvOS)
                    .frame(width: 160, height: 206)
                    #else
                    .frame(width: 100, height: 129)
                    #endif
                    .accessibilityLabel("Kinosail Player")
            }
            .ignoresSafeArea()
            .opacity(showingLaunch ? 1 : 0)
            .allowsHitTesting(showingLaunch)
            .accessibilityHidden(!showingLaunch)
        }
        .task {
            do { try await Task.sleep(for: .milliseconds(250)) }
            catch { return }
            launchMinimumElapsed = true
        }
        .onChange(of: launchMinimumElapsed && !session.restoring) { _, ready in
            guard ready else { return }
            if reduceMotion {
                showingLaunch = false
            } else {
                withAnimation(.easeOut(duration: 0.45)) { showingLaunch = false }
            }
        }
    }
}
