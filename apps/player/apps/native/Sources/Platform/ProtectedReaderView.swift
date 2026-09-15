#if os(iOS)
import Foundation
import PDFKit
import WebKit

@MainActor
final class ProtectedReaderView: UIView, WKNavigationDelegate, UIScrollViewDelegate, PDFViewDelegate {
    var onOffset: (Double) -> Void = { _ in }
    var onFailure: (String) -> Void = { _ in }
    var onNavigate: (URL) -> Void = { _ in }
    private var web: WKWebView?
    private var pdf: PDFView?
    private var page: ReaderBook.Page?
    private var client: ServerClient?
    private var itemID = ""
    private var identity = ""
    private var loader = ReaderResourceLoader()
    private var tasks: [ObjectIdentifier: Task<Void, Never>] = [:]
    private var fileTask: Task<Void, Never>?
    private var generation = UUID()
    private var observer: NSObjectProtocol?
    private var sizeObserver: NSKeyValueObservation?
    private var initialOffset: Double = 0
    private var restoring = true
    private var contentReady = false
    private var lastReport = Date.distantPast
    private var fontSize = 20
    private var theme = ReaderTheme.light

    func load(book: ReaderBook, page: ReaderBook.Page, offset: Double, fontSize: Int, theme: ReaderTheme, revision: UUID, client: ServerClient) {
        let key = "\(client.identity):\(page.resource):\(fontSize):\(theme):\(revision)"
        guard key != identity else { return }
        guard offset.isFinite, (0...1).contains(offset), (16...96).contains(fontSize) else { onFailure("The reading position or text size is invalid."); return }
        close()
        identity = key; self.page = page; self.client = client; itemID = book.id
        self.fontSize = fontSize; self.theme = theme; initialOffset = offset; restoring = true; contentReady = false
        let attempt = generation
        loader = ReaderResourceLoader()
        if book.kind == .pdf {
            fileTask = Task { [weak self] in
                guard let self else { return }
                do {
                    let (data, mime) = try await self.loader.load(url: page.resource, itemID: book.id, client: client)
                    try Task.checkCancellation()
                    guard self.generation == attempt else { return }
                    guard mime == "application/pdf", let document = PDFDocument(data: data), !document.isLocked,
                          document.pageCount > 0, document.pageCount <= 10_000 else { throw ClientError.invalidInput("This PDF is locked, empty, or too large to read.") }
                    let view = PDFView(frame: self.bounds)
                    view.delegate = self; view.autoScales = true; view.displayMode = .singlePageContinuous; view.displayDirection = .vertical
                    view.document = document; view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
                    view.backgroundColor = self.background
                    self.pdf = view; self.addSubview(view); self.contentReady = true
                    self.restorePosition()
                    self.observer = NotificationCenter.default.addObserver(forName: .PDFViewPageChanged, object: view, queue: .main) { [weak self, weak view] _ in
                        Task { @MainActor in
                            guard let self, let view, !self.restoring, let current = view.currentPage, let document = view.document else { return }
                            self.onOffset(Double(document.index(for: current)) / Double(max(1, document.pageCount - 1)))
                        }
                    }
                } catch is CancellationError {} catch { if self.generation == attempt { self.onFailure(AppSession.message(error)) } }
            }
            return
        }
        let configuration = WKWebViewConfiguration()
        configuration.websiteDataStore = .nonPersistent()
        configuration.defaultWebpagePreferences.allowsContentJavaScript = false
        let handler = ReaderSchemeHandler(); handler.owner = self
        configuration.setURLSchemeHandler(handler, forURLScheme: "kinoreader")
        let rules = #"[{"trigger":{"url-filter":".*"},"action":{"type":"block"}},{"trigger":{"url-filter":"^kinoreader://book/"},"action":{"type":"ignore-previous-rules"}}]"#
        WKContentRuleListStore.default().compileContentRuleList(forIdentifier: "KinosailSwiftReaderLocalOnly", encodedContentRuleList: rules) { [weak self] list, _ in
            guard let self, self.generation == attempt else { return }
            guard let list else { self.onFailure("Could not open the reader securely. Reopen this book to try again."); return }
            configuration.userContentController.add(list)
            let view = WKWebView(frame: self.bounds, configuration: configuration)
            view.navigationDelegate = self; view.scrollView.delegate = self
            view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
            view.isOpaque = false; view.backgroundColor = self.background; view.scrollView.backgroundColor = self.background
            self.web = view; self.addSubview(view)
            self.sizeObserver = view.scrollView.observe(\.contentSize, options: [.new]) { [weak self] _, _ in Task { @MainActor in self?.restorePosition() } }
            do { view.load(URLRequest(url: try ReaderResourcePolicy.local(page.resource))) }
            catch { self.onFailure(AppSession.message(error)) }
        }
    }

    override func layoutSubviews() { super.layoutSubviews(); restorePosition() }
    private var background: UIColor { theme == .dark ? UIColor(white: 0.07, alpha: 1) : theme == .sepia ? UIColor(red: 0.95, green: 0.91, blue: 0.80, alpha: 1) : .white }
    private func restorePosition() {
        guard restoring, contentReady, bounds.height > 0 else { return }
        if let pdf, let document = pdf.document, let page = document.page(at: Int((Double(max(0, document.pageCount - 1)) * initialOffset).rounded())) {
            pdf.go(to: page); restoring = false
        } else if let scroll = web?.scrollView, scroll.contentSize.height > 0 {
            scroll.setContentOffset(CGPoint(x: 0, y: max(0, scroll.contentSize.height - scroll.bounds.height) * initialOffset), animated: false)
        }
    }

    func start(_ request: any WKURLSchemeTask) {
        let key = ObjectIdentifier(request)
        guard let local = request.request.url, local.scheme == "kinoreader", local.host == "book", local.port == nil,
              local.user == nil, local.password == nil, local.query == nil, let client,
              let path = URLComponents(url: local, resolvingAgainstBaseURL: false)?.percentEncodedPath,
              tasks.count < 128 else { request.didFailWithError(ClientError.invalidResponse); return }
        let remote: URL
        do { remote = try ReaderResourcePolicy.remote(path, itemID: itemID, server: client.server) }
        catch { request.didFailWithError(error); return }
        let loader = loader, bookID = itemID, attempt = generation
        tasks[key] = Task { [weak self] in
            do {
                let (bytes, type) = try await loader.load(url: remote, itemID: bookID, client: client)
                try Task.checkCancellation()
                guard let self, self.generation == attempt, self.tasks[key] != nil else { return }
                var data = bytes, mime = type
                if mime.contains("html") {
                    guard bytes.count <= 8 * 1024 * 1024, let html = String(data: bytes, encoding: .utf8) else { throw ClientError.invalidResponse }
                    let colors = self.theme == .dark ? ("#121212", "#ececec") : self.theme == .sepia ? ("#f2e7cc", "#342b20") : ("#ffffff", "#202020")
                    let policy = "default-src 'none'; script-src 'none'; img-src kinoreader:; style-src kinoreader: 'unsafe-inline'; font-src kinoreader:; base-uri 'none'; form-action 'none'; frame-src 'none'; object-src 'none'; connect-src 'none'"
                    let head = "<meta http-equiv=\"Content-Security-Policy\" content=\"\(policy)\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><style>html,body{background:\(colors.0)!important;color:\(colors.1)!important;font-size:\(self.fontSize)px!important;line-height:1.65!important;}body{padding:16px!important;}img,svg{max-width:100%!important;height:auto!important;}a{color:inherit;}</style>"
                    data = Data((head + html).utf8); mime = "text/html"
                }
                self.tasks[key] = nil
                request.didReceive(URLResponse(url: local, mimeType: mime, expectedContentLength: data.count, textEncodingName: mime.hasPrefix("text/") ? "utf-8" : nil))
                request.didReceive(data); request.didFinish()
            } catch is CancellationError {} catch {
                guard let self, self.generation == attempt, self.tasks.removeValue(forKey: key) != nil else { return }
                request.didFailWithError(error)
                self.onFailure(AppSession.message(error))
            }
        }
    }
    func stop(_ task: any WKURLSchemeTask) { tasks.removeValue(forKey: ObjectIdentifier(task))?.cancel() }
    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { contentReady = true; restorePosition() }
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: any Error) { if (error as NSError).code != NSURLErrorCancelled { onFailure("The chapter could not be displayed.") } }
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: any Error) { if (error as NSError).code != NSURLErrorCancelled { onFailure("The chapter could not be opened.") } }
    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) { onFailure("The chapter stopped responding. Reopen it to continue reading.") }
    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction, decisionHandler: @escaping @MainActor @Sendable (WKNavigationActionPolicy) -> Void) {
        guard let url = navigationAction.request.url, url.scheme == "kinoreader", url.host == "book", url.query == nil,
              let client, let path = URLComponents(url: url, resolvingAgainstBaseURL: false)?.percentEncodedPath,
              let remote = try? ReaderResourcePolicy.remote(path, itemID: itemID, server: client.server) else { decisionHandler(.cancel); return }
        if navigationAction.targetFrame?.isMainFrame == true, remote != page?.resource { onNavigate(remote); decisionHandler(.cancel); return }
        decisionHandler(.allow)
    }
    func scrollViewWillBeginDragging(_ scrollView: UIScrollView) { restoring = false }
    func scrollViewDidScroll(_ scrollView: UIScrollView) {
        if !restoring, Date().timeIntervalSince(lastReport) >= 1 { report(scrollView) }
    }
    func scrollViewDidEndDragging(_ scrollView: UIScrollView, willDecelerate decelerate: Bool) { if !decelerate { report(scrollView) } }
    func scrollViewDidEndDecelerating(_ scrollView: UIScrollView) { report(scrollView) }
    private func report(_ scrollView: UIScrollView) {
        guard !restoring, contentReady, scrollView.bounds.height > 0 else { return }
        lastReport = Date()
        let travel = max(0, scrollView.contentSize.height - scrollView.bounds.height)
        onOffset(travel > 0 ? min(1, max(0, scrollView.contentOffset.y / travel)) : 0)
    }
    // PDF links are delegated instead of opening the browser or another file.
    func pdfViewWillClick(onLink sender: PDFView, with url: URL) { onFailure("External links cannot open in the reader. You can still follow links within this document.") }
    func pdfViewOpenPDF(_ sender: PDFView, forRemoteGoToAction action: PDFActionRemoteGoTo) { onFailure("This link points outside the current document.") }

    func close() {
        generation = UUID(); identity = ""
        tasks.values.forEach { $0.cancel() }; tasks = [:]; fileTask?.cancel(); fileTask = nil
        if let observer { NotificationCenter.default.removeObserver(observer) }; observer = nil; sizeObserver = nil
        web?.stopLoading(); web?.navigationDelegate = nil; web?.scrollView.delegate = nil; web?.removeFromSuperview(); web = nil
        pdf?.delegate = nil; pdf?.removeFromSuperview(); pdf = nil
        client = nil; page = nil
    }
}

@MainActor
private final class ReaderSchemeHandler: NSObject, WKURLSchemeHandler {
    weak var owner: ProtectedReaderView?
    func webView(_ webView: WKWebView, start urlSchemeTask: any WKURLSchemeTask) { owner?.start(urlSchemeTask) }
    func webView(_ webView: WKWebView, stop urlSchemeTask: any WKURLSchemeTask) { owner?.stop(urlSchemeTask) }
}
#endif
