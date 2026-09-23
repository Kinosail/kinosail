import SwiftUI

struct SupporterScreen: View {
    @Environment(AppSession.self) private var session
    @State private var collection: SupporterCollection?
    @State private var error: String?
    @State private var saving = false
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 28) {
                Text("A place in the story.").font(.largeTitle.bold())
                Text("Collect one-time, monthly and yearly badges. Kinosail stays complete and free for everyone.")
                if let collection {
                    ForEach(["one-time", "monthly", "yearly"], id: \.self) { edition in
                        let badge = collection.badges.first { $0.edition == edition }
                        HStack(spacing: 24) {
                            Image(badge?.artwork ?? "supporter-\(edition)-1").resizable().scaledToFit().frame(width: 112, height: 112)
                            VStack(alignment: .leading) {
                                Text(badge?.title ?? ["one-time": "One-time", "monthly": "Monthly", "yearly": "Yearly"][edition]!).font(.headline)
                                Text(badge?.name ?? "Not yet collected")
                                if let badge { Text(badge.archived ? "Past support · Yours to keep" : "Collected").font(.caption) }
                            }
                        }
                    }
                    ForEach(collection.badges.filter { $0.edition == "legacy" }) { badge in
                        Text("Earlier support · \(badge.name)\(badge.archived ? " · Past support" : "")")
                    }
                    if session.viewer?.owner == true {
                        Toggle("Show supporter badges around the app", isOn: Binding(get: { collection.visible }, set: save))
                            .disabled(saving)
                        Text("Applies across this Server. Your collection always stays here.").font(.footnote).foregroundStyle(.secondary)
                    }
                }
                if let error { Text(error).foregroundStyle(.secondary); Button("Try again") { Task { await load() } } }
                Text("Activate or refresh a supporter key from Supporter on your Server’s web app.").font(.footnote)
            }.padding(32)
        }.background(KinoTheme.background).navigationTitle("Supporter")
            .task(id: session.supporterRevision) { await load() }
    }
    private func load() async {
        do { collection = try await session.client?.supporterCollection(); error = nil }
        catch { self.error = error.localizedDescription }
    }
    private func save(_ visible: Bool) {
        saving = true
        Task {
            do { try await session.client?.setSupporterVisibility(visible); session.supporterRevision = UUID(); await load() }
            catch { self.error = error.localizedDescription }
            saving = false
        }
    }
}

struct SupporterToolbar: ViewModifier {
    @Environment(AppSession.self) private var session
    @Environment(\.scenePhase) private var phase
    @State private var collection: SupporterCollection?
    @State private var showing = false
    func body(content: Content) -> some View {
        content.toolbar {
            if let collection, collection.visible {
                ToolbarItem(placement: .automatic) {
                    Button { showing = true } label: {
                        if collection.badges.isEmpty { Text("Support Kinosail").font(.caption) }
                        else {
                            HStack(spacing: 5) {
                                ForEach(collection.badges.filter { $0.edition != "legacy" }) { badge in
                                    Image("supporter-\(badge.edition)-small").resizable().scaledToFit().frame(width: 24, height: 24)
                                }
                                if collection.badges.allSatisfy({ $0.edition == "legacy" }) { Image(systemName: "sailboat") }
                            }
                        }
                    }.accessibilityLabel(collection.badges.isEmpty ? "Support Kinosail" : "Your supporter collection")
                }
            }
        }
        .sheet(isPresented: $showing, onDismiss: { Task { await load() } }) { NavigationStack { SupporterScreen() } }
        .task(id: session.supporterRevision) { await load() }
        .onChange(of: phase) { _, phase in if phase == .active { Task { await load() } } }
    }
    private func load() async { collection = try? await session.client?.supporterCollection() }
}
