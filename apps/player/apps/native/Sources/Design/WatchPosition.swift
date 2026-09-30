import SwiftUI

struct WatchPosition: View {
    let item: MediaItem
    var compact = false
    var barOnly = false
    @Environment(\.dynamicTypeSize) private var dynamicType
    @Environment(AppSession.self) private var session
    @State private var progress: WatchProgressSummary?
    @State private var loadedIdentity: String?
    private var cacheKey: String { "watch-progress:\(item.id)" }
    private var requestIdentity: String { "\(session.client?.identity.uuidString ?? ""):\(item.id):\(session.contentRevision)" }
    private var visibleProgress: WatchProgressSummary? {
        guard item.kind == .video, let clientID = session.client?.identity else { return nil }
        if loadedIdentity == requestIdentity { return progress }
        guard let saved = session.resourceSnapshots.value(for: cacheKey, clientID: clientID, as: WatchProgressSummary.self) else { return nil }
        return try? WatchProgressSummary(.object(["seconds": .number(item.progress.seconds), "duration": .number(saved.duration)]))
    }
    var body: some View {
        let progress = visibleProgress
        Group {
            if barOnly {
                Color.clear.frame(height: 6).overlay {
                    if let fraction = progress?.fraction {
                        ProgressView(value: fraction).tint(KinoTheme.signal)
                            .progressViewStyle(.linear)
                            .background(.black.opacity(0.78), in: Capsule())
                            .accessibilityLabel("Watch progress")
                            .accessibilityValue("\(Int((fraction * 100).rounded())) percent")
                    }
                }
            } else {
                let layout = compact || dynamicType.isAccessibilitySize
                    ? AnyLayout(VStackLayout(alignment: .leading, spacing: 4))
                    : AnyLayout(HStackLayout(alignment: .center, spacing: 12))
                layout {
                    if compact { remaining }
                    if let fraction = progress?.fraction {
                        ProgressView(value: fraction).tint(KinoTheme.signal).accessibilityLabel("Watch progress")
                            .accessibilityValue(progress?.remainingLabel ?? "")
                    }
                    if !compact { remaining }
                }
            }
        }
        .task(id: requestIdentity) {
            guard item.kind == .video, let client = session.client else { return }
            let identity = requestIdentity, revision = session.contentRevision.uuidString
            let clientID = client.identity
            if session.resourceSnapshots.isFresh(for: cacheKey, clientID: clientID, as: WatchProgressSummary.self, refreshID: revision) { return }
            do {
                if let saved = try? await client.watchProgress(itemID: item.id, policy: .cached) {
                    try Task.checkCancellation()
                    guard requestIdentity == identity else { return }
                    session.resourceSnapshots.store(saved, for: cacheKey, clientID: clientID)
                }
                let hasSaved = session.resourceSnapshots.value(for: cacheKey, clientID: clientID, as: WatchProgressSummary.self) != nil
                let next = try await client.watchProgress(itemID: item.id, policy: hasSaved ? .reload : .automatic)
                try Task.checkCancellation()
                guard requestIdentity == identity else { return }
                session.resourceSnapshots.store(next, for: cacheKey, clientID: clientID, refreshID: revision)
                self.progress = next
                loadedIdentity = identity
            } catch {
                if requestIdentity == identity, (error as? ClientError)?.discardsCachedContent == true {
                    session.resourceSnapshots.remove(for: cacheKey, clientID: clientID, as: WatchProgressSummary.self)
                    self.progress = nil
                    loadedIdentity = identity
                }
            }
        }
    }
    private var remaining: some View {
        Text(visibleProgress?.remainingLabel ?? "Continue from \(item.progress.seconds.clock)")
            .font(.caption).foregroundStyle(KinoTheme.muted).monospacedDigit()
            .fixedSize(horizontal: !compact && !dynamicType.isAccessibilitySize, vertical: true)
            .layoutPriority(1)
    }
}
