import Foundation
import TVServices

final class ContentProvider: TVTopShelfContentProvider {
    override func loadTopShelfContent() async -> (any TVTopShelfContent)? {
        guard let snapshot = try? ShelfSnapshot.read(), let root = ShelfSnapshot.directory() else { return nil }
        var sections: [TVTopShelfItemCollection<TVTopShelfSectionedItem>] = []
        for title in ["Continue watching", "My List", "Recently added"] {
            var items: [TVTopShelfSectionedItem] = []
            for entry in snapshot.items where entry.section == title {
                guard let play = snapshot.url(for: entry, play: true), let detail = snapshot.url(for: entry, play: false) else { continue }
                let file = root.appendingPathComponent("\(entry.id).jpg")
                guard file.resolvingSymlinksInPath() == file.standardizedFileURL else { continue }
                do { try entry.image.write(to: file, options: .atomic) } catch { continue }
                let item = TVTopShelfSectionedItem(identifier: title + ":" + entry.id)
                item.title = entry.title
                item.imageShape = .poster
                item.setImageURL(file, for: .screenScale1x)
                item.playAction = TVTopShelfAction(url: play)
                item.displayAction = TVTopShelfAction(url: detail)
                items.append(item)
            }
            if !items.isEmpty {
                let section = TVTopShelfItemCollection(items: items)
                section.title = title
                sections.append(section)
            }
        }
        // A sign-out or privacy change may have removed/replaced the snapshot while loading.
        guard let current = try? ShelfSnapshot.read(), current.scope == snapshot.scope,
              current.saved == snapshot.saved else { return nil }
        return sections.isEmpty ? nil : TVTopShelfSectionedContent(sections: sections)
    }
}
