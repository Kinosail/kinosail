import SwiftUI

struct TabPreferencesScreen: View {
    @Environment(AppSession.self) private var session
    var body: some View { TabPreferencesEditor(profileKey: session.profileKey ?? "") }
}

private struct TabPreferencesEditor: View {
    @AppStorage private var watchStored: String
    @AppStorage private var listenStored: String
    @AppStorage private var modeStored: String
    @State private var message: String?
    init(profileKey: String) {
        let legacy = UserDefaults.standard.string(forKey: "kinosail.tabs.\(profileKey)")
        _watchStored = AppStorage(wrappedValue: PlayerTab.legacyDefault(legacy), PlayerMode.watch.tabsKey(profileKey))
        _listenStored = AppStorage(wrappedValue: PlayerMode.listen.defaultTabs.map(\.rawValue).joined(separator: ","), PlayerMode.listen.tabsKey(profileKey))
        _modeStored = AppStorage(wrappedValue: PlayerMode.watch.rawValue, PlayerMode.storageKey(profileKey))
    }
    private var mode: PlayerMode {
        #if os(iOS)
        PlayerMode.stored(modeStored)
        #else
        .watch
        #endif
    }
    private var pinned: [PlayerTab] { (try? PlayerTab.parse(mode == .watch ? watchStored : listenStored)) ?? mode.defaultTabs }
    private var intro: String {
        #if os(iOS)
        "Choose up to four \(mode.title) tabs for this Viewer Profile on this device. Find the remaining sections in More."
        #else
        "Choose up to four tabs for this Viewer Profile on this device. Find the remaining sections in More."
        #endif
    }
    var body: some View {
        List {
            Section {
                Text(intro)
                    .foregroundStyle(KinoTheme.muted)
                if let message { Text(message).foregroundStyle(KinoTheme.text) }
            }
            Section("Your tabs") {
                ForEach(pinned) { tab in
                    VStack(alignment: .leading, spacing: 8) {
                        Label(tab.title, systemImage: tab.symbol).fixedSize(horizontal: false, vertical: true)
                        HStack(spacing: 8) {
                            Button { move(tab, by: -1) } label: { Image(systemName: "arrow.up").frame(minWidth: 44, minHeight: 44).contentShape(Rectangle()) }
                                .accessibilityLabel("Move \(tab.title) earlier").disabled(pinned.first == tab)
                            Button { move(tab, by: 1) } label: { Image(systemName: "arrow.down").frame(minWidth: 44, minHeight: 44).contentShape(Rectangle()) }
                                .accessibilityLabel("Move \(tab.title) later").disabled(pinned.last == tab)
                            Button { save(pinned.filter { $0 != tab }) } label: { Image(systemName: "minus.circle").frame(minWidth: 44, minHeight: 44).contentShape(Rectangle()) }
                                .accessibilityLabel("Remove \(tab.title) from tabs").disabled(pinned.count == 1)
                        }
                    }.buttonStyle(.borderless)
                }
            }
            Section("Add a tab") {
                ForEach(PlayerTab.available.filter { !pinned.contains($0) }) { tab in
                    Button { save(pinned + [tab]) } label: { Label(tab.title, systemImage: tab.symbol) }
                        .disabled(pinned.count == 4)
                }
            }
            Section { Button("Reset to default tabs") { save(mode.defaultTabs) } }
        }
        #if os(iOS)
        .scrollContentBackground(.hidden)
        #endif
        .background(KinoTheme.background)
        .navigationTitle(mode == .listen ? "Customize Listen tabs" : "Customize tabs")
        .tvOSConfigurationLayout(title: "Customize tabs", symbol: "rectangle.3.group")
        .onChange(of: mode) { _, _ in message = nil }
    }
    private func save(_ items: [PlayerTab]) {
        do {
            let raw = items.map(\.rawValue).joined(separator: ",")
            _ = try PlayerTab.parse(raw)
            if mode == .watch { watchStored = raw } else { listenStored = raw }
            message = nil
        } catch { message = AppSession.message(error) }
    }
    private func move(_ tab: PlayerTab, by offset: Int) {
        var items = pinned
        guard let index = items.firstIndex(of: tab), items.indices.contains(index + offset) else { return }
        items.swapAt(index, index + offset)
        save(items)
    }
}
