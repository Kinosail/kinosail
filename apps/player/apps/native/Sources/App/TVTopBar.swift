#if os(tvOS)
import SwiftUI

struct TVTopBar: View {
    let focus: FocusState<PlayerTab?>.Binding
    let onSelect: (PlayerTab) -> Void

    var body: some View {
        HStack(spacing: 18) {
            Button { onSelect(.search) } label: {
                Label("Search", systemImage: "magnifyingglass")
                    .padding(.horizontal, 18).frame(height: 52)
            }
            .focused(focus, equals: .search)
            .buttonStyle(.plain)
            .foregroundStyle(focus.wrappedValue == .search ? KinoTheme.text : KinoTheme.muted)
            .background(focus.wrappedValue == .search ? KinoTheme.raised : KinoTheme.surface, in: Capsule())
            Spacer()
            Button { onSelect(.settings) } label: { Image(systemName: "gearshape").frame(width: 52, height: 52) }
                .focused(focus, equals: .settings)
                .buttonStyle(.plain)
                .foregroundStyle(focus.wrappedValue == .settings ? KinoTheme.text : KinoTheme.muted)
                .background(focus.wrappedValue == .settings ? KinoTheme.raised : KinoTheme.surface, in: RoundedRectangle(cornerRadius: 16))
                .accessibilityLabel("Settings")
        }
        .font(.callout)
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, KinoTheme.contentPadding)
        .padding(.vertical, 12)
        .focusSection()
    }
}
#endif
