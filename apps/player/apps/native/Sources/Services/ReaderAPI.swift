import Foundation

extension ServerClient {
    func reader(itemID: String) async throws -> ReaderBook {
        let id = try Input.id(itemID)
        return try ReaderBook(await request("/api/v1/books/\(id)/reader").body, itemID: id, server: server)
    }
    func readerPosition(itemID: String) async throws -> ReaderPosition {
        try ReaderPosition(await request("/api/v1/books/\(Input.id(itemID))/reader/progress?includeOffset=true").body)
    }
    func saveReaderPosition(itemID: String, page: Int, offset: Double) async throws -> ReaderPosition {
        let id = try Input.id(itemID)
        guard (1...10_000).contains(page), offset.isFinite, (0...1).contains(offset) else { throw ClientError.invalidInput("The reading position is invalid.") }
        let result = try ReaderPosition(await request("/api/v1/books/\(id)/reader/progress?includeOffset=true", method: .put,
                                                      body: .object(["page": .number(Double(page)), "offset": .number(offset)])).body)
        guard result.page == page, result.offset == offset else { throw ClientError.invalidResponse }
        return result
    }
}
