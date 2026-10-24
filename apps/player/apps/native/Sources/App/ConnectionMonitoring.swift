import SwiftUI
import UIKit

struct ConnectionMonitoring: ViewModifier {
    @Environment(AppSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase

    func body(content: Content) -> some View {
        content
            .task(id: "\(session.client?.identity.uuidString ?? ""):\(scenePhase)") {
                guard scenePhase == .active, let client = session.client else { return }
                await session.resumeDeviceLibrary()
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
            #if os(iOS)
            .onChange(of: DownloadNetwork(online: session.connection.networkAvailable, expensive: session.connection.expensive,
                                             server: session.connection.server != .unreachable), initial: true) { _, network in
                Task { await session.downloads.updateNetwork(network) }
            }
            .onChange(of: scenePhase, initial: true) { _, phase in
                session.downloads.engine.setForeground(phase == .active)
                guard phase == .active else { return }
                Task { await session.downloads.resumeInterrupted() }
            }
            #endif
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
                    #if os(iOS)
                    await session.downloads.resumeInterrupted()
                    #endif
                }
            }
    }
}
