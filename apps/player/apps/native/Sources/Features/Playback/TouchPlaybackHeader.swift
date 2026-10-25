#if os(iOS)
import SwiftUI

struct TouchPlaybackHeader: View {
    let item: MediaItem?
    let compact: Bool
    let close: () -> Void
    let options: () -> Void

    private var subtitle: String {
        guard let item else { return "" }
        if !item.show.isEmpty { return "\(item.show) · S\(item.season) E\(item.episode)" }
        return item.year
    }

    var body: some View {
        HStack(spacing: 12) {
            Button("Close player", systemImage: "chevron.down", action: close)
                .labelStyle(.iconOnly).font(.body.weight(.semibold)).frame(width: 44, height: 44)
                .glassEffect(.regular.interactive(), in: .circle)
                .keyboardShortcut(.cancelAction)
            VStack(alignment: .leading, spacing: 2) {
                Text(item?.title ?? "Playback")
                    .font(.headline).lineLimit(compact ? 1 : 2)
                    .accessibilityAddTraits(.isHeader)
                if !subtitle.isEmpty, !compact {
                    Text(subtitle).font(.caption.weight(.medium)).foregroundStyle(.white.opacity(0.7)).lineLimit(1)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            Button("Playback options", systemImage: "ellipsis", action: options)
                .labelStyle(.iconOnly).font(.body.weight(.semibold)).frame(width: 44, height: 44)
                .glassEffect(.regular.interactive(), in: .circle)
        }
        .buttonStyle(.plain)
        .padding(.horizontal, 16).padding(.vertical, 8)
        .background(LinearGradient(colors: [.black.opacity(0.75), .clear], startPoint: .top, endPoint: .bottom).ignoresSafeArea(edges: .top))
    }
}
#endif
