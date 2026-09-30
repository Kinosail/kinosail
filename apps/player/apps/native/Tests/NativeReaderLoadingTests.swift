#if os(iOS)
import PDFKit
import Testing
import UIKit
@testable import KinosailPlayer

@Suite(.serialized) @MainActor
struct NativeReaderLoadingTests {
    @Test func documentLoadingIsVisibleAndStopsOnSuccessFailureAndClose() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let path = "/read/book/file"
        let server = await fixture.client.server
        let book = try ReaderBook(.object(["id": .string("book"), "title": .string("Book"), "type": .string("pdf"),
            "pages": .array([.object(["number": .number(1), "title": .string("Page"), "url": .string(path)])])]), itemID: "book", server: server)
        let view = ProtectedReaderView(frame: CGRect(x: 0, y: 0, width: 400, height: 600))
        var failure: String?
        view.onFailure = { failure = $0 }
        defer { view.close() }
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.routes[path] = .init(data: Data(), status: 200, headers: ["Content-Type": "application/pdf"], hold: true) }
        view.load(book: book, page: book.pages[0], offset: 0, fontSize: 20, theme: .dark, revision: UUID(), client: fixture.client)
        try await until { fixture.requests.contains { $0.url?.path == path } }
        #expect(view.subviews.compactMap { $0 as? UIActivityIndicatorView }.contains { $0.isAnimating })
        view.close()
        #expect(!view.subviews.compactMap { $0 as? UIActivityIndicatorView }.contains { $0.isAnimating })
        for (data, status) in [(Data("unavailable".utf8), 503), (Data("invalid PDF".utf8), 200)] {
            failure = nil
            FixtureURLProtocol.entries.withLock { $0[fixture.host]?.routes[path] = .init(data: data, status: status, headers: ["Content-Type": "application/pdf"]) }
            view.load(book: book, page: book.pages[0], offset: 0, fontSize: 20, theme: .dark, revision: UUID(), client: fixture.client)
            try await until { failure != nil }
            #expect(!view.subviews.compactMap { $0 as? UIActivityIndicatorView }.contains { $0.isAnimating })
        }
        let pdf = UIGraphicsPDFRenderer(bounds: CGRect(x: 0, y: 0, width: 400, height: 600)).pdfData { context in context.beginPage(); ("A quiet morning" as NSString).draw(at: CGPoint(x: 20, y: 40), withAttributes: [.font: UIFont.systemFont(ofSize: 20)]) }
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.routes[path] = .init(data: pdf, status: 200, headers: ["Content-Type": "application/pdf"]) }
        view.load(book: book, page: book.pages[0], offset: 0, fontSize: 20, theme: .dark, revision: UUID(), client: fixture.client)
        try await until { view.subviews.compactMap { $0 as? PDFView }.first?.document?.pageCount == 1 }
        #expect(!view.subviews.compactMap { $0 as? UIActivityIndicatorView }.contains { $0.isAnimating })
        await fixture.client.close()
    }
    private func until(_ condition: () -> Bool) async throws {
        let deadline = Date().addingTimeInterval(5)
        while !condition() && Date() < deadline { try await Task.sleep(for: .milliseconds(10)) }
        try #require(condition())
    }
}
#endif
