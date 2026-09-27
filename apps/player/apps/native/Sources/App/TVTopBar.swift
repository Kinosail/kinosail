#if os(tvOS)
import SwiftUI

struct TVTopBar: View {
    @Binding var selection: PlayerTab
    let focus: FocusState<PlayerTab?>.Binding

    var body: some View {
        HStack(spacing: 18) {
            if selection == .home {
                Button { selection = .search } label: {
                    Label("Search", systemImage: "magnifyingglass")
                        .padding(.horizontal, 18).frame(height: 52)
                }
                    .focused(focus, equals: .search)
                    .buttonStyle(.plain)
                    .foregroundStyle(focus.wrappedValue == .search ? KinoTheme.text : KinoTheme.muted)
                    .background(focus.wrappedValue == .search ? KinoTheme.raised : KinoTheme.surface, in: Capsule())
                Spacer()
                Button { selection = .library } label: { Image(systemName: "square.grid.2x2").frame(width: 52, height: 52) }
                    .focused(focus, equals: .library)
                    .buttonStyle(.plain)
                    .foregroundStyle(focus.wrappedValue == .library ? KinoTheme.text : KinoTheme.muted)
                    .background(focus.wrappedValue == .library ? KinoTheme.raised : KinoTheme.surface, in: RoundedRectangle(cornerRadius: 16))
                    .accessibilityLabel("Library")
                Button { selection = .settings } label: { Image(systemName: "gearshape").frame(width: 52, height: 52) }
                    .focused(focus, equals: .settings)
                    .buttonStyle(.plain)
                    .foregroundStyle(focus.wrappedValue == .settings ? KinoTheme.text : KinoTheme.muted)
                    .background(focus.wrappedValue == .settings ? KinoTheme.raised : KinoTheme.surface, in: RoundedRectangle(cornerRadius: 16))
                    .accessibilityLabel("Settings")
            } else {
                Image(systemName: "chevron.left")
                    .opacity(focus.wrappedValue == nil || focus.wrappedValue == .home ? 0.3 : 1)
                    .accessibilityHidden(true)
                ScrollViewReader { proxy in
                    ScrollView(.horizontal) {
                        HStack(spacing: 8) {
                            ForEach(PlayerTab.tvBrowse) { tab in
                                Button { selection = tab } label: {
                                    Text(tab.title)
                                        .padding(.horizontal, 16)
                                        .padding(.vertical, 12)
                                }
                                    .focused(focus, equals: tab)
                                    .buttonStyle(.plain)
                                    .foregroundStyle(selection == tab ? KinoTheme.signal : KinoTheme.text)
                                    .background(focus.wrappedValue == tab ? KinoTheme.raised : Color.clear, in: Capsule())
                                    .accessibilityHint("Move left or right to browse sections")
                                    .id(tab)
                            }
                        }
                    }
                    .scrollIndicators(.hidden)
                    .onChange(of: focus.wrappedValue) { _, tab in
                        if let tab, PlayerTab.tvBrowse.contains(tab) {
                            withAnimation(.easeInOut(duration: 0.2)) { proxy.scrollTo(tab, anchor: .center) }
                        }
                    }
                }
                .frame(maxWidth: 760)
                Image(systemName: "chevron.right")
                    .opacity(focus.wrappedValue == .library ? 0.3 : 1)
                    .accessibilityHidden(true)
                ForEach(PlayerTab.tvUtilities) { tab in
                    Button { selection = tab } label: {
                        Image(systemName: tab.symbol)
                            .frame(width: 52, height: 52)
                    }
                    .focused(focus, equals: tab)
                    .buttonStyle(.plain)
                    .foregroundStyle(selection == tab ? KinoTheme.signal : KinoTheme.text)
                    .background(focus.wrappedValue == tab ? KinoTheme.raised : KinoTheme.surface, in: RoundedRectangle(cornerRadius: 18))
                    .accessibilityLabel(tab.title)
                }
            }
        }
        .font(selection == .home ? .callout : .title3)
        .foregroundStyle(KinoTheme.muted)
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, KinoTheme.contentPadding)
        .padding(.vertical, 12)
        .background(KinoTheme.background)
        .focusSection()
        .onChange(of: focus.wrappedValue) { _, tab in
            if selection != .home, let tab, tab != .home { selection = tab }
        }
    }
}
#endif
