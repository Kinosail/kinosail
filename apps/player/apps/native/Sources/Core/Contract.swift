import Foundation
import Darwin

enum ClientError: LocalizedError, Equatable {
    case invalidInput(String), invalidResponse, http(Int), unavailable, secureStorage, discovery

    var permitsOfflineRestore: Bool {
        switch self {
        case .unavailable, .http(408), .http(429), .http(500), .http(502), .http(503), .http(504): true
        default: false
        }
    }

    var errorDescription: String? {
        switch self {
        case .secureStorage: "Secure session storage is unavailable. Unlock your device and try again."
        case .discovery: "Could not search for nearby Servers. Enter your Server address to connect."
        case .invalidInput(let message): message
        case .invalidResponse: "Could not read the Server’s response. Try again."
        case .unavailable: "Could not reach Kinosail Server. Check your connection and try again."
        case .http(401): "Your session has expired. Connect to your Server again."
        case .http(403): "This Viewer Profile cannot use that feature."
        case .http(404): "The requested title is no longer available."
        case .http(409): "This information changed on the Server. Refresh and try again."
        case .http(429): "Too many requests. Wait a minute and try again."
        case .http: "Kinosail Server could not complete the request."
        }
    }
}

// Decode remote JSON into Sendable values before it crosses actor boundaries.
// Every consumed field is subsequently checked by its domain initializer.
enum JSONValue: Codable, Sendable, Equatable {
    case object([String: JSONValue]), array([JSONValue]), string(String), number(Double), bool(Bool), null

    init(from decoder: any Decoder) throws {
        let value = try decoder.singleValueContainer()
        if value.decodeNil() { self = .null }
        else if let flag = try? value.decode(Bool.self) { self = .bool(flag) }
        else if let number = try? value.decode(Double.self), number.isFinite { self = .number(number) }
        else if let string = try? value.decode(String.self) { self = .string(string) }
        else if let array = try? value.decode([JSONValue].self) { self = .array(array) }
        else { self = .object(try value.decode([String: JSONValue].self)) }
    }

    func encode(to encoder: any Encoder) throws {
        var value = encoder.singleValueContainer()
        switch self {
        case .object(let item): try value.encode(item)
        case .array(let item): try value.encode(item)
        case .string(let item): try value.encode(item)
        case .number(let item): try value.encode(item)
        case .bool(let item): try value.encode(item)
        case .null: try value.encodeNil()
        }
    }

    func object(allowing keys: Set<String>? = nil) throws -> [String: JSONValue] {
        guard case .object(let value) = self,
              keys.map({ Set(value.keys).isSubset(of: $0) }) ?? true else { throw ClientError.invalidResponse }
        return value
    }

    func array(max: Int) throws -> [JSONValue] {
        if case .null = self { return [] }
        guard case .array(let value) = self, value.count <= max else { throw ClientError.invalidResponse }
        return value
    }
}

extension Dictionary where Key == String, Value == JSONValue {
    func text(_ key: String, max: Int = 2048, required: Bool = false) throws -> String {
        guard let raw = self[key] else {
            if required { throw ClientError.invalidResponse }
            return ""
        }
        guard case .string(let value) = raw, value.utf8.count <= max,
              !required || !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
              !value.unicodeScalars.contains(where: { CharacterSet.controlCharacters.subtracting(.newlines).contains($0) })
        else { throw ClientError.invalidResponse }
        return value
    }

    func number(_ key: String, max: Double = 31_536_000, fallback: Double = 0, integer: Bool = false) throws -> Double {
        guard let raw = self[key] else { return fallback }
        guard case .number(let value) = raw, value.isFinite, value >= 0, value <= max,
              !integer || value.rounded() == value else { throw ClientError.invalidResponse }
        return value
    }

    func flag(_ key: String, fallback: Bool? = nil) throws -> Bool {
        guard let raw = self[key] else {
            if let fallback { return fallback }
            throw ClientError.invalidResponse
        }
        guard case .bool(let value) = raw else { throw ClientError.invalidResponse }
        return value
    }

    func list(_ key: String, max: Int) throws -> [JSONValue] {
        try (self[key] ?? .array([])).array(max: max)
    }
}

enum Input {
    static func text(_ value: String, max: Int, label: String, empty: Bool = false) throws -> String {
        let normalized = value.trimmingCharacters(in: .whitespacesAndNewlines)
        guard value.utf8.count <= max, empty || !normalized.isEmpty,
              value.rangeOfCharacter(from: .controlCharacters) == nil else {
            throw ClientError.invalidInput("Enter a valid \(label).")
        }
        return normalized
    }

    static func id(_ value: String) throws -> String {
        guard value.range(of: "\\A[A-Za-z0-9_-]{1,128}\\z", options: .regularExpression) != nil else {
            throw ClientError.invalidResponse
        }
        return value
    }

    static func code(_ value: String) throws -> String {
        guard value.range(of: "\\A[0-9]{6}\\z", options: .regularExpression) != nil else {
            throw ClientError.invalidInput("Enter the six-digit code shown on your TV.")
        }
        return value
    }

    static func secret(_ value: String) throws -> String {
        guard value.range(of: "\\A[^\\x00-\\x20\\x7f]{1,2048}\\z", options: .regularExpression) != nil else {
            throw ClientError.invalidInput("The saved device session is invalid.")
        }
        return value
    }

    static func position(_ value: Double) throws {
        guard value.isFinite, (0...31_536_000).contains(value) else {
            throw ClientError.invalidInput("The playback position is invalid.")
        }
    }
}

struct ServerAddress: Codable, Hashable, Sendable {
    let url: URL

    init(_ input: String) throws {
        let raw = input.trimmingCharacters(in: .whitespacesAndNewlines)
        guard raw.utf8.count <= 2048, !raw.isEmpty, !raw.contains("\\"),
              raw.rangeOfCharacter(from: .controlCharacters) == nil,
              var parts = URLComponents(string: raw),
              let scheme = parts.scheme?.lowercased(), let host = parts.host?.lowercased(), !host.isEmpty,
              Self.validHost(host),
              host.rangeOfCharacter(from: .whitespacesAndNewlines.union(.controlCharacters)) == nil,
              scheme == "https" || scheme == "http" && Self.localHost(host),
              parts.user == nil, parts.password == nil, parts.query == nil, parts.fragment == nil,
              parts.path.isEmpty || parts.path == "/", parts.port.map({ (1...65535).contains($0) }) ?? true
        else { throw ClientError.invalidInput("Enter a valid Kinosail Server URL. Remote connections require HTTPS.") }
        parts.scheme = scheme
        parts.host = host
        parts.path = ""
        if parts.port == (scheme == "https" ? 443 : 80) { parts.port = nil }
        guard let url = parts.url else { throw ClientError.invalidResponse }
        self.url = url
    }

    init(from decoder: any Decoder) throws {
        try self.init(decoder.singleValueContainer().decode(String.self))
    }

    func encode(to encoder: any Encoder) throws {
        var value = encoder.singleValueContainer()
        try value.encode(url.absoluteString)
    }

    func mediaURL(_ path: String) throws -> URL {
        guard !path.isEmpty, path.utf8.count <= 16_384, !path.contains("\\"),
              path.rangeOfCharacter(from: .controlCharacters) == nil,
              let value = URL(string: path, relativeTo: url)?.absoluteURL,
              let base = URLComponents(url: url, resolvingAgainstBaseURL: false),
              let result = URLComponents(url: value, resolvingAgainstBaseURL: true),
              result.scheme?.lowercased() == base.scheme, result.host?.lowercased() == base.host,
              (result.port ?? (result.scheme == "https" ? 443 : 80)) == (base.port ?? (base.scheme == "https" ? 443 : 80)),
              result.user == nil, result.password == nil, result.fragment == nil,
              !result.path.contains("\\"), result.path.rangeOfCharacter(from: .controlCharacters) == nil,
              !result.path.split(separator: "/", omittingEmptySubsequences: false).contains(where: { $0 == "." || $0 == ".." }),
              result.percentEncodedPath.range(of: "%2f|%5c", options: [.regularExpression, .caseInsensitive]) == nil else { throw ClientError.invalidResponse }
        return value
    }

    static func localHost(_ input: String) -> Bool {
        let host = input.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "[]"))
        if host == "localhost" || host.hasSuffix(".local") || host == "::1" { return true }
        if host.contains(":") {
            guard let bytes = ipv6(host) else { return false }
            return bytes[0] & 0xfe == 0xfc || bytes[0] == 0xfe && bytes[1] & 0xc0 == 0x80
        }
        let segments = host.split(separator: ".", omittingEmptySubsequences: false)
        let parts = segments.compactMap { UInt8($0) }
        guard parts.count == 4, segments.enumerated().allSatisfy({ String(parts[$0.offset]) == $0.element }) else { return false }
        return parts[0] == 10 || parts[0] == 127 || parts[0] == 169 && parts[1] == 254 ||
            parts[0] == 172 && (16...31).contains(parts[1]) || parts[0] == 192 && parts[1] == 168
    }

    private static func ipv6(_ host: String) -> [UInt8]? {
        var address = in6_addr()
        guard host.withCString({ inet_pton(AF_INET6, $0, &address) }) == 1 else { return nil }
        return withUnsafeBytes(of: &address) { Array($0) }
    }

    private static func validHost(_ input: String) -> Bool {
        let host = input.trimmingCharacters(in: CharacterSet(charactersIn: "[]"))
        if host.contains(":") { return ipv6(host) != nil }
        guard host.utf8.count <= 253 else { return false }
        let labels = host.split(separator: ".", omittingEmptySubsequences: false)
        guard !labels.isEmpty, labels.allSatisfy({ label in
            !label.isEmpty && label.utf8.count <= 63 && label.first != "-" && label.last != "-" &&
                label.utf8.allSatisfy { (97...122).contains($0) || (48...57).contains($0) || $0 == 45 }
        }) else { return false }
        if labels.last?.allSatisfy(\.isNumber) == true || host.hasPrefix("0x") {
            guard labels.count == 4 else { return false }
            return labels.allSatisfy { value in UInt8(value).map { String($0) == value } ?? false }
        }
        return true
    }
}
