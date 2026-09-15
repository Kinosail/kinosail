import SwiftUI

struct WatchPosition: View {
    let item: MediaItem
    var compact = false
    @Environment(\.dynamicTypeSize) private var dynamicType
    @Environment(AppSession.self) private var session
    @State private var progress: WatchProgressSummary?
    var body: some View {
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
        .task(id: "\(session.profileKey ?? ""):\(item.id):\(session.contentRevision)") {
            progress = nil
            guard item.kind == .video, let client = session.client else { return }
            do {
                let next = try await client.watchProgress(itemID: item.id)
                try Task.checkCancellation()
                progress = next
            } catch { /* Saved position remains useful when duration is unavailable. */ }
        }
    }
    private var remaining: some View {
        Text(progress?.remainingLabel ?? "Continue from \(item.progress.seconds.clock)")
            .font(.caption).foregroundStyle(KinoTheme.muted).monospacedDigit()
            .fixedSize(horizontal: !compact && !dynamicType.isAccessibilitySize, vertical: true)
            .layoutPriority(1)
    }
}
