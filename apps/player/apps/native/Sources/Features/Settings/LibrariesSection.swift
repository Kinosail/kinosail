import SwiftUI

struct LibrariesSection: View {
    @Environment(AppSession.self) private var session
    @State private var sources: [SourceAccount] = []
    @State private var removing: SourceAccount?
    @State private var failure: String?

    var body: some View {
        Section {
            ForEach(sources) { source in
                #if os(tvOS)
                Button { removing = source } label: { row(source) }
                #else
                row(source)
                    .swipeActions { Button("Remove", role: .destructive) { removing = source } }
                    .contextMenu { Button("Remove", systemImage: "trash", role: .destructive) { removing = source } }
                #endif
            }
            Button("Add a server", systemImage: "plus") { session.showsSetup = true }
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

    private func row(_ source: SourceAccount) -> some View {
        HStack(spacing: 14) {
            ServerMonogram(brand: .source(source.kind), size: 40)
            VStack(alignment: .leading, spacing: 4) {
                Text(source.name).font(.headline).foregroundStyle(KinoTheme.text)
                Text(source.name == source.kind.title ? "Signed in as \(source.userName)" : "\(source.kind.title) · \(source.userName)").font(.subheadline).foregroundStyle(KinoTheme.muted)
            }
        }
        .padding(.vertical, 4)
        .accessibilityElement(children: .combine)
    }
}
