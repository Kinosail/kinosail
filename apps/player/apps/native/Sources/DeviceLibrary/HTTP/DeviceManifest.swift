import Foundation

enum DeviceManifest {
    static func rewrite(_ data: Data, rewrite: DeviceManifestRewrite) throws -> Data {
        guard data.count <= 2 * 1024 * 1024, let raw = String(data: data, encoding: .utf8),
              raw.hasPrefix("#EXTM3U") else { throw DeviceFailure.invalid }
        let lines = raw.components(separatedBy: .newlines)
        guard lines.count <= 65_536 else { throw DeviceFailure.invalid }
        func mapped(_ reference: String) throws -> String {
            guard !reference.isEmpty, reference.utf8.count <= 16_384,
                  let resolved = URL(string: reference, relativeTo: rewrite.base)?.absoluteURL,
                  resolved.scheme == rewrite.base.scheme, resolved.host == rewrite.base.host, resolved.port == rewrite.base.port
            else { throw DeviceFailure.invalid }
            return try rewrite.register(resolved)
        }
        let uri = try NSRegularExpression(pattern: #"URI="([^"]*)""#)
        var output: [String] = []
        output.reserveCapacity(lines.count)
        for rawLine in lines {
            guard rawLine.utf8.count <= 32_768, !rawLine.contains("#EXT-X-DEFINE") else { throw DeviceFailure.invalid }
            var line = rawLine.trimmingCharacters(in: .whitespaces)
            if !line.isEmpty, !line.hasPrefix("#") { line = try mapped(line) }
            else if line.contains("URI=\"") {
                let matches = uri.matches(in: line, range: NSRange(line.startIndex..., in: line))
                guard matches.count <= 16 else { throw DeviceFailure.invalid }
                for match in matches.reversed() {
                    guard let range = Range(match.range(at: 1), in: line) else { throw DeviceFailure.invalid }
                    line.replaceSubrange(range, with: try mapped(String(line[range])))
                }
            }
            output.append(line)
        }
        return Data(output.joined(separator: "\n").utf8)
    }
}

final class DeviceStreamRegistry: @unchecked Sendable {
    private let lock = NSLock()
    private var resources: [String: (item: String, request: URLRequest)] = [:]
    private var paths: [URL: String] = [:]
    private var order: [String] = []

    func register(_ url: URL, itemID: String, template: URLRequest) throws -> String {
        lock.lock(); defer { lock.unlock() }
        if let existing = paths[url] { return existing }
        let name = UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased()
        let playlist = url.path.lowercased().hasSuffix(".m3u8")
        let path = "/hls/\(itemID)/\(name)\(playlist ? ".m3u8" : ".ts")"
        var request = template
        request.url = url
        request.httpMethod = "GET"
        request.httpBody = nil
        resources[path] = (itemID, request)
        paths[url] = path
        order.append(path)
        if order.count > 40_000 {
            let evicted = order.removeFirst()
            if let gone = resources.removeValue(forKey: evicted), let url = gone.request.url { paths.removeValue(forKey: url) }
        }
        return path
    }

    func resolve(_ path: String, itemID: String) -> URLRequest? {
        lock.lock(); defer { lock.unlock() }
        guard let entry = resources[path], entry.item == itemID else { return nil }
        return entry.request
    }
}
