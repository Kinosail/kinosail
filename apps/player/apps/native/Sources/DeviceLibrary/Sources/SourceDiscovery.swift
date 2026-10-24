import Darwin
import Foundation

enum SourceDiscovery {
    static let ports = [8096, 32400]
    private static let session: URLSession = {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpCookieStorage = nil
        configuration.urlCredentialStorage = nil
        configuration.urlCache = nil
        configuration.timeoutIntervalForRequest = 1.5
        configuration.timeoutIntervalForResource = 3
        configuration.httpMaximumConnectionsPerHost = 2
        configuration.waitsForConnectivity = false
        return URLSession(configuration: configuration, delegate: HTTPPolicy(), delegateQueue: nil)
    }()

    static func scan(found: @escaping @MainActor (SourceServerInfo) -> Void) async {
        let hosts = candidates()
        guard !hosts.isEmpty else { return }
        await withTaskGroup(of: SourceServerInfo?.self) { group in
            var pending = hosts.flatMap { host in ports.compactMap { URL(string: "http://\(host):\($0)") } }.makeIterator()
            func next() -> Bool {
                guard let address = pending.next() else { return false }
                group.addTask { address.port == 32400 ? try? await PlexRoutes.probe(address, session: session) : try? await SourceConnect.probe(address, session: session) }
                return true
            }
            for _ in 0..<48 where next() {}
            var seen = Set<String>()
            while let result = await group.next() {
                if Task.isCancelled { group.cancelAll(); return }
                if let result, seen.insert(result.id).inserted { await found(result) }
                _ = next()
            }
        }
    }

    static func identify(_ input: String) async -> SourceServerInfo? {
        if let url = try? SourceAddress.parse(input, defaultPort: 8096), let server = try? await SourceConnect.probe(url) { return server }
        if let url = try? SourceAddress.parse(input, defaultPort: 32400), let server = try? await PlexRoutes.probe(url) { return server }
        return nil
    }

    static func candidates() -> [String] {
        var head: UnsafeMutablePointer<ifaddrs>?
        guard getifaddrs(&head) == 0, let first = head else { return [] }
        defer { freeifaddrs(head) }
        var hosts: [String] = []
        var seen = Set<UInt32>()
        for pointer in sequence(first: first, next: { $0.pointee.ifa_next }) {
            let entry = pointer.pointee
            let flags = Int32(entry.ifa_flags)
            guard flags & IFF_UP != 0, flags & IFF_LOOPBACK == 0, let address = entry.ifa_addr, let mask = entry.ifa_netmask,
                  address.pointee.sa_family == UInt8(AF_INET) else { continue }
            let name = String(cString: entry.ifa_name)
            guard name.hasPrefix("en") || name.hasPrefix("bridge") else { continue }
            let ip = address.withMemoryRebound(to: sockaddr_in.self, capacity: 1) { UInt32(bigEndian: $0.pointee.sin_addr.s_addr) }
            let netmask = mask.withMemoryRebound(to: sockaddr_in.self, capacity: 1) { UInt32(bigEndian: $0.pointee.sin_addr.s_addr) }
            guard netmask >= 0xFFFF_0000, Self.privateRange(ip) else { continue }
            let base = ip & 0xFFFF_FF00
            guard seen.insert(base).inserted else { continue }
            for host in 1...254 where base | UInt32(host) != ip { hosts.append(Self.text(base | UInt32(host))) }
            hosts.insert(Self.text(ip), at: 0)
        }
        return Array(hosts.prefix(1024))
    }

    private static func privateRange(_ ip: UInt32) -> Bool {
        ip >> 24 == 10 || ip >> 20 == 0xAC1 || ip >> 16 == 0xC0A8
    }

    private static func text(_ ip: UInt32) -> String {
        "\(ip >> 24).\((ip >> 16) & 0xFF).\((ip >> 8) & 0xFF).\(ip & 0xFF)"
    }
}
