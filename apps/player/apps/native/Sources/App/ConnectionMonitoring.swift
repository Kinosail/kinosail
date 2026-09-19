import SwiftUI
import UIKit

struct ConnectionMonitoring: ViewModifier {
    @Environment(AppSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase

    func body(content: Content) -> some View {
        content
            .task(id: "\(session.client?.identity.uuidString ?? ""):\(scenePhase)") {
                guard scenePhase == .active, let client = session.client else { return }
                await session.connection.monitor(client)
            }
            .task(id: "\(session.client?.identity.uuidString ?? ""):\(String(describing: session.connection.networkAvailable)):\(scenePhase)") {
                guard scenePhase == .active, let client = session.client else { return }
                do { try await Task.sleep(for: .seconds(1)) }
                catch { return }
                await session.connection.check(client)
                while !Task.isCancelled {
                    do { try await Task.sleep(for: .seconds(15)) }
                    catch { return }
                    if session.connection.unavailable { await session.connection.check(client) }
                }
            }
            .onChange(of: session.connection.unavailable) { wasUnavailable, unavailable in
                if unavailable {
                    UIAccessibility.post(notification: .announcement, argument: session.connection.title)
                    return
                }
                guard wasUnavailable, !unavailable, session.connection.server == .reachable, let client = session.client else { return }
                UIAccessibility.post(notification: .announcement, argument: "Connected to your Server again.")
                Task {
                    await client.invalidateCatalog()
                    guard session.client?.identity == client.identity else { return }
                    session.contentRevision = UUID()
                }
            }
    }
}
