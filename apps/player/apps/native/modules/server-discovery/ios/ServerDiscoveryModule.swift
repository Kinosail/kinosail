import ExpoModulesCore
import Foundation

public final class ServerDiscoveryModule: Module {
  private let discovery = ServerBrowser()

  public func definition() -> ModuleDefinition {
    Name("ServerDiscovery")
    AsyncFunction("scan") { (promise: Promise) in
      self.discovery.scan(promise)
    }.runOnQueue(.main)
    AsyncFunction("stop") {
      self.discovery.finish()
    }.runOnQueue(.main)
    OnDestroy {
      DispatchQueue.main.async { self.discovery.finish() }
    }
  }
}

// All Bonjour callbacks and lifecycle changes run on the main run loop.
private final class ServerBrowser: NSObject, NetServiceBrowserDelegate, NetServiceDelegate {
  private var browser: NetServiceBrowser?
  private var services: [NetService] = []
  private var results: [String: [String: String]] = [:]
  private var promise: Promise?
  private var timer: Timer?

  func scan(_ promise: Promise) {
    finish()
    self.promise = promise
    let browser = NetServiceBrowser()
    self.browser = browser
    browser.delegate = self
    browser.searchForServices(ofType: "_kinosail-player._tcp.", inDomain: "local.")
    timer = Timer.scheduledTimer(withTimeInterval: 5, repeats: false) { [weak self] _ in self?.finish() }
  }

  func finish() {
    timer?.invalidate(); timer = nil
    browser?.delegate = nil; browser?.stop(); browser = nil
    for service in services { service.delegate = nil; service.stop() }
    services.removeAll()
    let pending = promise; promise = nil
    let snapshot = results.values.sorted { ($0["name"] ?? "") < ($1["name"] ?? "") }
    results.removeAll()
    pending?.resolve(snapshot)
  }

  func netServiceBrowser(_ browser: NetServiceBrowser, didNotSearch errorDict: [String: NSNumber]) {
    guard browser === self.browser else { return }
    let pending = promise; promise = nil
    finish()
    pending?.reject("ERR_DISCOVERY", "Local server discovery is unavailable.")
  }

  func netServiceBrowser(_ browser: NetServiceBrowser, didFind service: NetService, moreComing: Bool) {
    guard browser === self.browser, services.count < 32,
          !services.contains(where: { $0.name == service.name && $0.domain == service.domain }),
          !service.name.isEmpty, service.name.utf8.count <= 63,
          service.name.rangeOfCharacter(from: .controlCharacters) == nil else { return }
    services.append(service)
    service.delegate = self
    service.resolve(withTimeout: 3)
  }

  func netServiceBrowser(_ browser: NetServiceBrowser, didRemove service: NetService, moreComing: Bool) {
    guard browser === self.browser else { return }
    results.removeValue(forKey: service.name)
    services.filter { $0.name == service.name }.forEach { $0.delegate = nil; $0.stop() }
    services.removeAll { $0.name == service.name }
  }

  func netServiceDidResolveAddress(_ sender: NetService) {
    guard promise != nil, services.contains(where: { $0 === sender }),
          (1...65535).contains(sender.port),
          let data = sender.txtRecordData(), data.count <= 1024 else { return }
    let txt = NetService.dictionary(fromTXTRecord: data)
    guard txt.count >= 2, txt.count <= 3,
          Set(txt.keys).isSubset(of: ["version", "scheme", "url"]),
          txt["version"] == Data("1".utf8),
          let schemeData = txt["scheme"], let scheme = String(data: schemeData, encoding: .utf8),
          scheme == "http" || scheme == "https" else { return }
    let origin: String
    if let value = txt["url"] {
      guard value.count <= 240, let url = String(data: value, encoding: .utf8),
            url.hasPrefix(scheme + "://") else { return }
      origin = url
    } else {
      guard let resolved = sender.hostName else { return }
      let host = resolved.hasSuffix(".") ? String(resolved.dropLast()) : resolved
      guard host.utf8.count <= 253, host.lowercased().hasSuffix(".local"),
            host.range(of: "^[A-Za-z0-9.-]+$", options: .regularExpression) != nil else { return }
      origin = "\(scheme)://\(host):\(sender.port)"
    }
    // JavaScript applies the shared connection URL policy before displaying or using it.
    results[sender.name] = ["name": sender.name, "url": origin]
  }
}
