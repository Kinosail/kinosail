import ExpoModulesCore
#if os(iOS)
import WebKit
import PDFKit

public final class ProtectedReaderModule: Module {
  public func definition() -> ModuleDefinition {
    Name("ProtectedReader")
    View(ProtectedReaderView.self) {
      Events("onError", "onPosition")
      Prop("server") { (view: ProtectedReaderView, value: String) in view.server = value }
      Prop("authorization") { (view: ProtectedReaderView, value: String) in view.authorization = value }
      Prop("id") { (view: ProtectedReaderView, value: String) in view.bookID = value }
      Prop("path") { (view: ProtectedReaderView, value: String) in view.path = value }
      Prop("type") { (view: ProtectedReaderView, value: String) in view.kind = value }
      Prop("fontSize") { (view: ProtectedReaderView, value: Int) in view.fontSize = value }
      Prop("offset") { (view: ProtectedReaderView, value: Double) in
        guard value.isFinite, (0...1).contains(value) else { view.onError([:]); return }
        view.offset = value
      }
      Prop("theme") { (view: ProtectedReaderView, value: String) in view.theme = value }
      OnViewDidUpdateProps { (view: ProtectedReaderView) in view.load() }
    }
  }
}

final class ReaderRequest: NSObject, URLSessionDataDelegate {
  private var data = Data()
  private var response: URLResponse?
  private var session: URLSession?
  private let reserve: (Int) -> Bool
  private let finish: (Data?, URLResponse?) -> Void
  init(url: URL, authorization: String, reserve: @escaping (Int) -> Bool = { _ in true }, finish: @escaping (Data?, URLResponse?) -> Void) {
    self.finish = finish; self.reserve = reserve
    super.init()
    let config = URLSessionConfiguration.ephemeral
    config.urlCredentialStorage = nil; config.httpCookieStorage = nil; config.urlCache = nil
    config.httpShouldSetCookies = false; config.timeoutIntervalForResource = 60
    session = URLSession(configuration: config, delegate: self, delegateQueue: .main)
    var request = URLRequest(url: url)
    request.setValue(authorization, forHTTPHeaderField: "Authorization")
    session?.dataTask(with: request).resume()
  }
  func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse, newRequest request: URLRequest, completionHandler: @escaping (URLRequest?) -> Void) { completionHandler(nil) }
  func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive response: URLResponse, completionHandler: @escaping (URLSession.ResponseDisposition) -> Void) {
    guard (response as? HTTPURLResponse)?.statusCode == 200, response.expectedContentLength <= 32 * 1024 * 1024 else { completionHandler(.cancel); return }
    self.response = response; completionHandler(.allow)
  }
  func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
    guard self.data.count + data.count <= 32 * 1024 * 1024, reserve(data.count) else { dataTask.cancel(); return }
    self.data.append(data)
  }
  func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
    finish(error == nil ? data : nil, response); session.finishTasksAndInvalidate(); self.session = nil
  }
  func cancel() { session?.invalidateAndCancel(); session = nil }
}

final class WeakReaderHandler: NSObject, WKURLSchemeHandler {
  weak var owner: ProtectedReaderView?
  func webView(_ webView: WKWebView, start urlSchemeTask: WKURLSchemeTask) { owner?.webView(webView, start: urlSchemeTask) }
  func webView(_ webView: WKWebView, stop urlSchemeTask: WKURLSchemeTask) { owner?.webView(webView, stop: urlSchemeTask) }
}
final class ProtectedReaderView: ExpoView, WKURLSchemeHandler, WKNavigationDelegate, UIScrollViewDelegate {
  var server = "", authorization = "", bookID = "", path = "", kind = "", theme = "light"
  var fontSize = 20
  var offset: Double = 0
  let onPosition = EventDispatcher()
  private var restorePending = false
  private var contentReady = false
  private var lastReport: TimeInterval = 0
  private var sizeObserver: NSKeyValueObservation?
  private var pdfObserver: NSObjectProtocol?
  let onError = EventDispatcher()
  private var web: WKWebView?
  private var pdf: PDFView?
  private var loaded = ""
  private var requests: [ObjectIdentifier: ReaderRequest] = [:]
  private var fileRequest: ReaderRequest?
  private var totalBytes = 0
  private var generation = 0
  private func validPath(_ value: String) -> Bool {
    let prefix = "/read/" + bookID + "/"
    return value.utf8.count <= 2048 && (value == prefix + "file" || value.hasPrefix(prefix + "asset/")) &&
      !value.contains("..") && !value.contains("\\") && !value.contains("?") && !value.contains("#") &&
      !value.unicodeScalars.contains(where: CharacterSet.controlCharacters.contains)
  }
  override func layoutSubviews() {
    super.layoutSubviews()
    clipsToBounds = true
    web?.frame = bounds
    pdf?.frame = bounds
    restoreScroll()
  }
  private func restoreScroll() {
    guard restorePending, contentReady, let scroll = web?.scrollView, scroll.bounds.height > 0, scroll.contentSize.height > 0 else { return }
    let travel = max(0, scroll.contentSize.height - scroll.bounds.height)
    scroll.setContentOffset(CGPoint(x: 0, y: travel * CGFloat(offset)), animated: false)
  }
  func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
    contentReady = true
    restoreScroll()
  }
  func scrollViewWillBeginDragging(_ scrollView: UIScrollView) { restorePending = false }
  func scrollViewDidScroll(_ scrollView: UIScrollView) {
    guard scrollView.isDragging || scrollView.isDecelerating else { return }
    let now = Date.timeIntervalSinceReferenceDate
    if now - lastReport >= 1 { lastReport = now; report(scrollView) }
  }
  private func report(_ scrollView: UIScrollView) {
    guard contentReady, !restorePending, scrollView.bounds.height > 0 else { return }
    let travel = max(0, scrollView.contentSize.height - scrollView.bounds.height)
    let fraction = travel > 0 ? min(1, max(0, scrollView.contentOffset.y / travel)) : 1
    onPosition(["path": path, "offset": fraction])
  }
  func scrollViewDidEndDragging(_ scrollView: UIScrollView, willDecelerate decelerate: Bool) { report(scrollView) }
  func scrollViewDidEndDecelerating(_ scrollView: UIScrollView) { report(scrollView) }
  func scrollViewDidEndScrollingAnimation(_ scrollView: UIScrollView) {
    restorePending = false
    report(scrollView)
  }
  func load() {
    let key = [server, authorization, bookID, path, kind, theme, String(fontSize)].joined(separator: "\n")
    guard key != loaded else { return }
    guard server.utf8.count <= 2048, let origin = URL(string: server),
      origin.scheme == "https" || origin.scheme == "http" && approvedLocalHost(origin.host ?? ""),
      origin.host != nil, origin.user == nil, origin.password == nil, origin.query == nil, origin.fragment == nil,
      origin.path.isEmpty || origin.path == "/",
      bookID.range(of: "^[A-Za-z0-9_-]{1,128}$", options: .regularExpression) != nil,
      authorization.range(of: "^Bearer [^\\x00-\\x20\\x7f]{1,2048}$", options: .regularExpression) != nil,
      offset.isFinite, (0...1).contains(offset),
      validPath(path), ["epub", "pdf", "comic"].contains(kind), (16...32).contains(fontSize),
      ["light", "dark", "sepia"].contains(theme)
    else { onError([:]); return }
    clipsToBounds = true
    contentReady = false; restorePending = true
    sizeObserver = nil
    if let pdfObserver { NotificationCenter.default.removeObserver(pdfObserver) }; pdfObserver = nil
    loaded = key; generation += 1; let current = generation
    requests.values.forEach { $0.cancel() }; requests.removeAll(); fileRequest?.cancel()
    web?.stopLoading(); web?.removeFromSuperview(); web = nil; pdf?.removeFromSuperview(); pdf = nil; totalBytes = 0
    if kind == "pdf" {
      guard let url = URL(string: server + path) else { onError([:]); return }
      fileRequest = ReaderRequest(url: url, authorization: authorization) { [weak self] data, _ in
        guard let self, self.generation == current else { return }
        guard let data, let document = PDFDocument(data: data) else { self.onError([:]); return }
        let view = PDFView(frame: self.bounds); view.autoScales = true; view.document = document
        view.autoresizingMask = [.flexibleWidth, .flexibleHeight]; view.clipsToBounds = true; self.pdf = view; self.addSubview(view)
        if let page = document.page(at: Int((Double(max(0, document.pageCount - 1)) * self.offset).rounded())) { view.go(to: page) }
        self.pdfObserver = NotificationCenter.default.addObserver(forName: .PDFViewPageChanged, object: view, queue: .main) { [weak self, weak view] _ in
          guard let self, let view, let page = view.currentPage else { return }
          self.onPosition(["path": self.path, "offset": Double(document.index(for: page)) / Double(max(1, document.pageCount - 1))])
        }
      }
      return
    }
    let config = WKWebViewConfiguration()
    config.websiteDataStore = .nonPersistent()
    config.defaultWebpagePreferences.allowsContentJavaScript = false
    let handler = WeakReaderHandler(); handler.owner = self
    config.setURLSchemeHandler(handler, forURLScheme: "kinoreader")
    let rules = "[{\"trigger\":{\"url-filter\":\".*\",\"unless-domain\":[\"book\"]},\"action\":{\"type\":\"block\"}}]"
    WKContentRuleListStore.default().compileContentRuleList(forIdentifier: "KinosailReaderLocalOnly", encodedContentRuleList: rules) { [weak self] list, error in
      guard let self, self.generation == current else { return }
      guard let list, error == nil else { self.onError([:]); return }
      config.userContentController.add(list)
      let view = WKWebView(frame: self.bounds, configuration: config)
      view.navigationDelegate = self; view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
      view.clipsToBounds = true
      view.scrollView.delegate = self
      self.web = view; self.addSubview(view)
      self.sizeObserver = view.scrollView.observe(\.contentSize, options: [.new]) { [weak self] _, _ in self?.restoreScroll() }
      guard let url = URL(string: "kinoreader://book" + self.path) else { self.onError([:]); return }
      view.load(URLRequest(url: url))
    }
  }
  func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction, decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
    guard let url = navigationAction.request.url, url.scheme == "kinoreader", url.host == "book", validPath(url.path), url.query == nil else { decisionHandler(.cancel); return }
    guard navigationAction.targetFrame?.isMainFrame != true || url.path == path else { decisionHandler(.cancel); return }
    decisionHandler(.allow)
  }
  func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) { if (error as NSError).code != NSURLErrorCancelled { onError([:]) } }
  func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) { if (error as NSError).code != NSURLErrorCancelled { onError([:]) } }
  func webView(_ webView: WKWebView, start urlSchemeTask: WKURLSchemeTask) {
    let key = ObjectIdentifier(urlSchemeTask)
    guard let local = urlSchemeTask.request.url, local.scheme == "kinoreader", local.host == "book",
      local.user == nil, local.password == nil, local.query == nil, validPath(local.path),
      requests.count < 16, totalBytes < 64 * 1024 * 1024,
      let remote = URL(string: server + local.path)
    else { urlSchemeTask.didFailWithError(NSError(domain: "Reader", code: 1)); return }
    requests[key] = ReaderRequest(url: remote, authorization: authorization, reserve: { [weak self] count in
      guard let self, self.totalBytes + count <= 64 * 1024 * 1024 else { return false }; self.totalBytes += count; return true
    }) { [weak self] data, response in
      guard let self, self.requests.removeValue(forKey: key) != nil else { return }
      guard var data, let response else { urlSchemeTask.didFailWithError(NSError(domain: "Reader", code: 2)); self.onError([:]); return }
      var mime = response.mimeType ?? "application/octet-stream"
      if mime.contains("html"), let html = String(data: data, encoding: .utf8) {
        let colors = self.theme == "dark" ? ("#151719", "#eceae5") : self.theme == "sepia" ? ("#f2e7cc", "#342b20") : ("#ffffff", "#202020")
        let policy = "default-src 'none'; img-src kinoreader: data:; style-src kinoreader: 'unsafe-inline'; font-src kinoreader:; base-uri 'none'; form-action 'none'; frame-src 'none'"
        let head = "<meta http-equiv=\"Content-Security-Policy\" content=\"\(policy)\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><style>html,body{background:\(colors.0)!important;color:\(colors.1)!important;font-size:\(self.fontSize)px!important;line-height:1.6!important;}body{padding:16px!important;}img{max-width:100%!important;height:auto!important;}</style>"
        data = Data((head + html).utf8); mime = "text/html"
      }
      urlSchemeTask.didReceive(URLResponse(url: local, mimeType: mime, expectedContentLength: data.count, textEncodingName: "utf-8"))
      urlSchemeTask.didReceive(data); urlSchemeTask.didFinish()
    }
  }
  func webView(_ webView: WKWebView, stop urlSchemeTask: WKURLSchemeTask) { requests.removeValue(forKey: ObjectIdentifier(urlSchemeTask))?.cancel() }
  deinit { if let pdfObserver { NotificationCenter.default.removeObserver(pdfObserver) }; requests.values.forEach { $0.cancel() }; fileRequest?.cancel() }
}

#else
public final class ProtectedReaderModule: Module {
  public func definition() -> ModuleDefinition { Name("ProtectedReader") }
}
#endif
