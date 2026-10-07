#if os(iOS)
import SwiftUI

struct TouchPlaybackHeader: View {
    let title: String
    let compact: Bool
    let close: () -> Void
    let options: () -> Void

    var body: some View {
        HStack(spacing: 12) {
            Button("Close player", systemImage: "chevron.down", action: close)
                .labelStyle(.iconOnly).frame(width: 44, height: 44)
                .keyboardShortcut(.cancelAction)
            Text(title)
                .font(.headline).lineLimit(compact ? 1 : 2).frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityAddTraits(.isHeader)
            Button("Playback options", systemImage: "ellipsis", action: options)
                .labelStyle(.iconOnly).frame(width: 44, height: 44)
        }
        .buttonStyle(.plain)
        .padding(.horizontal, 16).padding(.vertical, 8)
        .background(LinearGradient(colors: [.black.opacity(0.8), .clear], startPoint: .top, endPoint: .bottom).ignoresSafeArea(edges: .top))
    }
}
#endif
