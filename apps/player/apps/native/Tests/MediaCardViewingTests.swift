import Testing
@testable import KinosailPlayer

struct MediaCardViewingTests {
    @Test(arguments: [
        ("video", 0.0, false, true),
        ("video", 44.0, false, true),
        ("video", 44.0, true, false),
        ("show", 0.0, false, true),
        ("music", 0.0, false, false),
        ("photo", 0.0, false, false)
    ])
    func marksOnlyUnfinishedWatchableCards(_ kind: String, _ seconds: Double, _ watched: Bool, _ expected: Bool) throws {
        let item = try MediaItem(.object([
            "id": .string("sample"), "kind": .string(kind), "title": .string("Sample"),
            "progress": .object(["seconds": .number(seconds), "watched": .bool(watched)])
        ]), server: ServerAddress("https://media.example"))
        #expect(item.isUnwatched == expected)
    }
}
