#if os(tvOS)
import SwiftUI
import TVServices

enum TopShelfPreferences {
    static let enabledKey = "kinosail.topShelf.enabled"
    private static let defaultOnMigrationKey = "kinosail.topShelf.defaultOnMigration.v1"

    static func migrateToDefaultOn(in defaults: UserDefaults = .standard) {
        guard !defaults.bool(forKey: defaultOnMigrationKey) else { return }
        if defaults.object(forKey: enabledKey) == nil { defaults.set(true, forKey: enabledKey) }
        defaults.set(true, forKey: defaultOnMigrationKey)
    }
}

struct TopShelfPublishing: ViewModifier {
    @Environment(AppSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase
    @AppStorage(TopShelfPreferences.enabledKey) private var enabled = true
    static func clear() {
        ShelfSnapshot.clear()
        TVTopShelfContentProvider.topShelfContentDidChange()
    }
    func body(content: Content) -> some View {
        content.task(id: "\(enabled):\(session.profileKey ?? ""):\(session.contentRevision):\(scenePhase)") {
            guard scenePhase == .active else { return }
            Self.clear()
            guard enabled, let client = session.client, let scope = session.profileKey else { return }
            do {
                _ = try await client.viewer()
                var items: [ShelfSnapshot.Item] = []
                for (view, sort, section) in [(LibraryView.history, LibrarySort.title, ShelfSnapshot.Section.continueWatching), (.list, .title, .myList), (.all, .added, .recentlyAdded)] {
                    let page = try await client.library(view: view, sort: sort, limit: 24)
                    for item in page.items.filter({ $0.kind == .video }).prefix(6) {
                        try Task.checkCancellation()
                        guard let image = try? await session.artwork.image(path: item.artwork, client: client, dimension: 400),
                              let data = UIImage(cgImage: image).jpegData(compressionQuality: 0.7), data.count <= ShelfSnapshot.maximumImage else { continue }
                        items.append(.init(id: item.id, title: item.title, section: section.rawValue, image: data))
                    }
                }
                try Task.checkCancellation()
                guard session.profileKey == scope, enabled else { return }
                try ShelfSnapshot(scope: scope, saved: Date(), items: items).write()
                TVTopShelfContentProvider.topShelfContentDidChange()
            } catch { /* No private catalog is exposed when authentication or refresh fails. */ }
        }
    }
}
#endif
