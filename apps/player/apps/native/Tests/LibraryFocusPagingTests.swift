import Testing
@testable import KinosailPlayer

struct LibraryFocusPagingTests {
    @Test func prefetchesOnlyNearTheEndOfAnIncompletePage() throws {
        let page = try libraryPage(offset: 0, total: 80)
        #expect(!LibraryFocusPaging.shouldLoadNextPage(focusedID: "item-15", items: page.items, page: page))
        #expect(LibraryFocusPaging.shouldLoadNextPage(focusedID: "item-16", items: page.items, page: page))
        #expect(LibraryFocusPaging.shouldLoadNextPage(focusedID: "item-39", items: page.items, page: page))
        #expect(!LibraryFocusPaging.shouldLoadNextPage(focusedID: "missing", items: page.items, page: page))
        #expect(!LibraryFocusPaging.shouldLoadNextPage(focusedID: "item-39", items: page.items, page: nil))
    }

    @Test func stopsAtTheFinalPageEvenAfterALetterJump() throws {
        let page = try libraryPage(offset: 40, total: 80)
        #expect(!LibraryFocusPaging.shouldLoadNextPage(focusedID: "item-39", items: page.items, page: page))
        let jumped = try libraryPage(offset: 40, total: 120)
        #expect(LibraryFocusPaging.shouldLoadNextPage(focusedID: "item-39", items: jumped.items, page: jumped))
        let empty = try libraryPage(offset: 40, total: 120, count: 0)
        #expect(!LibraryFocusPaging.shouldLoadNextPage(focusedID: "item-39", items: jumped.items, page: empty))
    }

    private func libraryPage(offset: Int, total: Int, count: Int = 40) throws -> LibraryPage {
        let items: [JSONValue] = (0..<count).map { index in
            .object(["id": .string("item-\(index)"), "kind": .string("video"), "title": .string("Item \(index)")])
        }
        return try LibraryPage(.object([
            "items": .array(items), "total": .number(Double(total)),
            "offset": .number(Double(offset)), "limit": .number(60), "letters": .array([])
        ]), server: ServerAddress("https://media.example"))
    }
}
