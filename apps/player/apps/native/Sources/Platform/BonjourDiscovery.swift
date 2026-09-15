import Foundation
import Observation

struct NearbyServer: Identifiable, Sendable {
    let name: String
    let address: ServerAddress
    var id: String { address.url.absoluteString }

    static func resolve(name: String, host: String?, port: Int, record: Data) throws -> NearbyServer {
        let name = try Input.text(name, max: 63, label: "Server name")
        guard (1...65535).contains(port), record.count <= 1024 else { throw ClientError.invalidResponse }
        let txt = NetService.dictionary(fromTXTRecord: record)
        guard (2...3).contains(txt.count), Set(txt.keys).isSubset(of: ["version", "scheme", "url"]),
              txt["version"] == Data("1".utf8), let schemeData = txt["scheme"],
              let scheme = String(data: schemeData, encoding: .utf8), ["http", "https"].contains(scheme)
        else { throw ClientError.invalidResponse }
        let origin: String
        if let data = txt["url"] {
            guard data.count <= 240, let value = String(data: data, encoding: .utf8),
                  value.hasPrefix(scheme + "://") else { throw ClientError.invalidResponse }
            origin = value
        } else {
            guard let host else { throw ClientError.invalidResponse }
            let normalized = host.hasSuffix(".") ? String(host.dropLast()) : host
            guard normalized.utf8.count <= 253, normalized.lowercased().hasSuffix(".local"),
                  normalized.range(of: "\\A[A-Za-z0-9.-]+\\z", options: .regularExpression) != nil
            else { throw ClientError.invalidResponse }
            origin = "\(scheme)://\(normalized):\(port)"
        }
        return try NearbyServer(name: name, address: ServerAddress(origin))
    }
}

// NetService delegates are scheduled on the main run loop. Swift 6's isolated
// conformances express that ownership without unchecked cross-thread access.
@MainActor @Observable
final class BonjourDiscovery: NSObject, @MainActor NetServiceBrowserDelegate, @MainActor NetServiceDelegate {
    private(set) var servers: [NearbyServer] = []
    private(set) var scanning = false
    private var browser: NetServiceBrowser?
    private var services: [NetService] = []
    private var failed = false
    private var generation = UUID()

    func scan() async throws {
        stop()
        servers = []
        failed = false
        scanning = true
        let attempt = UUID()
        generation = attempt
        let browser = NetServiceBrowser()
        self.browser = browser
        browser.delegate = self
        browser.searchForServices(ofType: "_kinosail-player._tcp.", inDomain: "local.")
        do { try await Task.sleep(for: .seconds(5)) }
        catch {
            if generation == attempt { stop() }
            throw error
        }
        guard generation == attempt else { return }
        stop()
        if failed { throw ClientError.discovery }
    }

    func stop() {
        generation = UUID()
        scanning = false
        browser?.delegate = nil
        browser?.stop()
        browser = nil
        for service in services { service.delegate = nil; service.stop() }
        services = []
    }

    func netServiceBrowser(_ browser: NetServiceBrowser, didNotSearch errorDict: [String: NSNumber]) {
        guard browser === self.browser else { return }
        failed = true
        browser.stop()
        scanning = false
    }

    func netServiceBrowser(_ browser: NetServiceBrowser, didFind service: NetService, moreComing: Bool) {
        guard browser === self.browser, services.count < 32,
              !services.contains(where: { $0.name == service.name && $0.domain == service.domain }),
              (try? Input.text(service.name, max: 63, label: "Server name")) != nil else { return }
        services.append(service)
        service.delegate = self
        service.resolve(withTimeout: 3)
    }

    func netServiceBrowser(_ browser: NetServiceBrowser, didRemove service: NetService, moreComing: Bool) {
        guard browser === self.browser else { return }
        servers.removeAll { $0.name == service.name }
        services.filter { $0.name == service.name }.forEach { $0.delegate = nil; $0.stop() }
        services.removeAll { $0.name == service.name }
    }

    func netServiceDidResolveAddress(_ sender: NetService) {
        guard scanning, services.contains(where: { $0 === sender }), let record = sender.txtRecordData(),
              let result = try? NearbyServer.resolve(name: sender.name, host: sender.hostName, port: sender.port, record: record)
        else { return }
        servers.removeAll { $0.id == result.id }
        servers.append(result)
        servers.sort { $0.name.localizedStandardCompare($1.name) == .orderedAscending }
    }
}
