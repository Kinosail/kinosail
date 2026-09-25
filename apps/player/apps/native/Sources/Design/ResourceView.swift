import SwiftUI

struct RetryState: View {
    var title = "Couldn’t load this content"
    let message: String
    let retry: () -> Void
    var body: some View {
        VStack(spacing: 16) {
            Label(title, systemImage: "exclamationmark.circle").font(.headline)
            Text(message).foregroundStyle(KinoTheme.muted).multilineTextAlignment(.center)
            Button("Try again", action: retry).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
        }
        .padding(32).frame(maxWidth: .infinity, minHeight: 220)
    }
}

struct ResourceView<Value: Sendable, Content: View>: View {
    let identity: String
    var refreshID = ""
    var loadingLayout = LoadingLayout.shelf
    var allowsPullToRefresh = true
    var revalidates = true
    let load: (CatalogPolicy) async throws -> Value
    @ViewBuilder let content: (Value) -> Content
    @Environment(AppSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase
    @State private var value: Value?
    @State private var failure: String?
    @State private var revision = 0
    @State private var generation = UUID()
    @State private var loadedIdentity: String?
    private var cacheRevision: String { refreshID.isEmpty ? session.contentRevision.uuidString : refreshID }

    private var savedValue: Value? {
        guard revalidates, let clientID = session.client?.identity else { return nil }
        return session.resourceSnapshots.value(for: identity, clientID: clientID)
    }

    var body: some View {
        Group {
            if allowsPullToRefresh {
                resourceContent.refreshable { await refresh(force: true) }
            } else {
                resourceContent
            }
        }
        .task(id: "\(identity):\(revalidates ? cacheRevision : "once"):\(revision):\(revalidates ? String(describing: scenePhase) : "once")") {
            guard !revalidates || scenePhase == .active else { return }
            await refresh(force: revision > 0)
            while revalidates && !Task.isCancelled {
                do { try await Task.sleep(for: .seconds(60)) }
                catch { return }
                await refresh(force: false)
            }
        }
    }

    private var resourceContent: some View {
        Group {
            if let value = (loadedIdentity == identity ? value : nil) ?? savedValue {
                VStack(alignment: .leading, spacing: 16) {
                    if loadedIdentity == identity, let failure {
                        Text("Couldn’t refresh. \(failure)").font(.callout).foregroundStyle(KinoTheme.muted)
                        Button("Try again") { revision += 1 }
                    }
                    content(value)
                }
            } else if loadedIdentity == identity, let failure { RetryState(message: failure) { revision += 1 } }
            else { LoadingState(layout: loadingLayout) }
        }
    }

    private func refresh(force: Bool) async {
        let clientID = session.client?.identity
        let expectedRevision = cacheRevision
        if loadedIdentity != identity { value = savedValue; failure = nil; loadedIdentity = identity }
        if revalidates, !force, let clientID,
           session.resourceSnapshots.isFresh(for: identity, clientID: clientID, as: Value.self, refreshID: expectedRevision) { return }
        let attempt = UUID()
        generation = attempt
        do {
            if value == nil, let saved = try? await load(.cached) {
                try Task.checkCancellation()
                guard generation == attempt, session.client?.identity == clientID,
                      (!revalidates || cacheRevision == expectedRevision) else { return }
                value = saved
                if revalidates, let clientID { session.resourceSnapshots.store(saved, for: identity, clientID: clientID) }
            }
            let next = try await load(force ? .reload : .automatic)
            try Task.checkCancellation()
            guard generation == attempt, session.client?.identity == clientID,
                  (!revalidates || cacheRevision == expectedRevision) else { return }
            value = next
            if revalidates, let clientID { session.resourceSnapshots.store(next, for: identity, clientID: clientID, refreshID: expectedRevision) }
            failure = nil
        } catch is CancellationError {}
        catch {
            if generation == attempt, session.client?.identity == clientID,
               (!revalidates || cacheRevision == expectedRevision) {
                if (error as? ClientError)?.discardsCachedContent == true {
                    value = nil
                    if let clientID { session.resourceSnapshots.remove(for: identity, clientID: clientID, as: Value.self) }
                    if error as? ClientError == .http(401) || error as? ClientError == .http(403) { session.resourceSnapshots.clear() }
                }
                failure = AppSession.message(error)
            }
        }
    }
}
