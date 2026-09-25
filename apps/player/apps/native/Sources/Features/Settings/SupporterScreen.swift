import SwiftUI

struct SupporterScreen: View {
    @Environment(AppSession.self) private var session
    @State private var collection: SupporterCollection?
    @State private var error: String?
    @State private var loading = true
    @State private var saving = false
    @State private var showingBadges = false
    private var badgeSize: CGFloat {
        #if os(tvOS)
        160
        #else
        112
        #endif
    }
    private var headingFont: Font {
        #if os(tvOS)
        .system(.largeTitle, design: .rounded).bold()
        #else
        .largeTitle.bold()
        #endif
    }
    private var pageWidth: CGFloat? {
        #if os(tvOS)
        1100
        #else
        nil
        #endif
    }
    private var pagePadding: CGFloat {
        #if os(tvOS)
        KinoTheme.contentPadding
        #else
        32
        #endif
    }
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 28) {
                Text("A place in the story.").font(headingFont).accessibilityAddTraits(.isHeader)
                Text("Your Server's existing badges appear here. Kinosail stays complete and free for everyone.")
                if loading && collection == nil { ProgressView("Loading collection…") }
                if let collection {
                    ForEach(collection.badges.filter { $0.edition != "legacy" }) { badge in
                        HStack(spacing: 24) {
                            Image(badge.artwork).resizable().scaledToFit().frame(width: badgeSize, height: badgeSize)
                            VStack(alignment: .leading) {
                                Text(badge.title).font(.headline)
                                Text(badge.name)
                                Text(badge.archived ? "Past support · Yours to keep" : "Collected").font(.caption)
                            }
                        }
                    }
                    ForEach(collection.badges.filter { $0.edition == "legacy" }) { badge in
                        Text("Earlier support · \(badge.name)\(badge.archived ? " · Past support" : "")")
                    }
                    if collection.badges.isEmpty { Text("No badges are recorded on this Server.").foregroundStyle(.secondary) }
                    if session.viewer?.owner == true && !collection.badges.isEmpty {
                        Toggle("Show supporter badges around the app", isOn: $showingBadges)
                            .disabled(saving)
                            .onChange(of: showingBadges) { _, visible in
                                if collection.visible != visible { save(visible) }
                            }
                        Text("Applies across this Server. Your collection always stays here.").font(.footnote).foregroundStyle(.secondary)
                    }
                }
                if let error { Text(error).foregroundStyle(.secondary); Button("Try again") { Task { await load() } } }
            }
            .frame(maxWidth: pageWidth, alignment: .leading)
            .frame(maxWidth: .infinity)
            .padding(pagePadding)
        }
        #if os(tvOS)
        .cinemaBackground()
        .navigationTitle("")
        #else
        .background(KinoTheme.background)
        .navigationTitle("Supporter")
        #endif
        .task(id: session.supporterRevision) { await load() }
    }
    private func load() async {
        loading = true
        error = nil
        defer { loading = false }
        do {
            guard let client = session.client else { throw ClientError.unavailable }
            collection = try await client.supporterCollection()
            showingBadges = collection?.visible ?? false
        }
        catch { self.error = AppSession.message(error) }
    }
    private func save(_ visible: Bool) {
        saving = true
        Task {
            do { try await session.client?.setSupporterVisibility(visible); session.supporterRevision = UUID(); await load() }
            catch { self.error = AppSession.message(error); showingBadges = collection?.visible ?? false }
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
            if let collection, collection.visible, !collection.badges.isEmpty {
                ToolbarItem(placement: .automatic) {
                    Button { showing = true } label: {
                        HStack(spacing: 5) {
                            ForEach(collection.badges.filter { $0.edition != "legacy" }) { badge in
                                Image("supporter-\(badge.edition)-small").resizable().scaledToFit().frame(width: 24, height: 24)
                            }
                            if collection.badges.allSatisfy({ $0.edition == "legacy" }) { Image(systemName: "sailboat") }
                        }
                    }.accessibilityLabel("Your supporter collection")
                }
            }
        }
        .sheet(isPresented: $showing, onDismiss: { Task { await load() } }) { NavigationStack { SupporterScreen() } }
        .task(id: session.supporterRevision) { await load() }
        .onChange(of: phase) { _, phase in if phase == .active { Task { await load() } } }
    }
    private func load() async { collection = try? await session.client?.supporterCollection() }
}
