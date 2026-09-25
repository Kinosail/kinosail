enum LibraryFocusPaging {
    static func shouldLoadNextPage(focusedID: String, items: [MediaItem], page: LibraryPage?) -> Bool {
        guard let page, !page.items.isEmpty, page.offset + page.items.count < page.total else { return false }
        return items.suffix(24).contains { $0.id == focusedID }
    }
}
