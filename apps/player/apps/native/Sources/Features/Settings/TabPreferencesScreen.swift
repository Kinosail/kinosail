import SwiftUI

struct TabPreferencesScreen: View {
    @Environment(AppSession.self) private var session
    var body: some View { TabPreferencesEditor(profileKey: session.profileKey ?? "") }
}

private struct TabPreferencesEditor: View {
    @AppStorage private var stored: String
    @State private var message: String?
    init(profileKey: String) {
        let legacy = UserDefaults.standard.string(forKey: "kinosail.tabs.\(profileKey)")
        _stored = AppStorage(wrappedValue: PlayerTab.legacyDefault(legacy), "kinosail.tabs.v2.\(profileKey)")
    }
    private var pinned: [PlayerTab] { (try? PlayerTab.parse(stored)) ?? PlayerTab.defaults }
    var body: some View {
        List {
            Section {
                Text("Choose up to four tabs for this Viewer Profile on this device. Find the remaining sections in More.")
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
            Section { Button("Reset to default tabs") { save(PlayerTab.defaults) } }
        }
        #if os(iOS)
        .scrollContentBackground(.hidden)
        #endif
        .background(KinoTheme.background).navigationTitle("Customize tabs")
    }
    private func save(_ items: [PlayerTab]) {
        do {
            let raw = items.map(\.rawValue).joined(separator: ",")
            _ = try PlayerTab.parse(raw)
            stored = raw
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
