import SwiftUI

struct LibrariesSection: View {
    @Environment(AppSession.self) private var session
    @State private var sources: [SourceAccount] = []
    @State private var removing: SourceAccount?
    @State private var failure: String?

    var body: some View {
        Section {
            ForEach(sources) { source in
                Label {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(source.name).font(.headline).foregroundStyle(KinoTheme.text)
                        Text("\(source.kind.title) · \(source.userName)").font(.subheadline).foregroundStyle(KinoTheme.muted)
                    }
                } icon: {
                    Image(systemName: "play.rectangle.on.rectangle").foregroundStyle(KinoTheme.signal)
                }
                .accessibilityElement(children: .combine)
                .swipeActions { Button("Remove", role: .destructive) { removing = source } }
                .contextMenu { Button("Remove", systemImage: "trash", role: .destructive) { removing = source } }
            }
            Button("Add a library", systemImage: "plus") { session.showsSetup = true }
            Button("Refresh libraries", systemImage: "arrow.clockwise") {
                Task {
                    await DeviceLibraryHost.shared.refresh()
                    session.resourceSnapshots.clear()
                    session.contentRevision = UUID()
                }
            }
            if let failure { Text(failure).font(.footnote).foregroundStyle(.red) }
        } header: { Text("Libraries") } footer: {
            Text("Titles from every library appear together. Kinosail Server adds downloads, bookmarks and casting.")
        }
        .task(id: session.contentRevision) { sources = DeviceLibraryHost.shared.sources }
        .alert("Remove \(removing?.name ?? "this library")?", isPresented: Binding(get: { removing != nil }, set: { if !$0 { removing = nil } })) {
            Button("Remove", role: .destructive) {
                guard let source = removing else { return }
                removing = nil
                Task {
                    do { try await session.removeLibrary(source.id); sources = DeviceLibraryHost.shared.sources }
                    catch { failure = AppSession.message(error) }
                }
            }
            Button("Cancel", role: .cancel) { removing = nil }
        } message: { Text("Its titles leave this device. Nothing changes on the server.") }
    }
}
