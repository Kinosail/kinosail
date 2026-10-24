import Foundation
import UIKit

enum SourceHTTP {
    static let session: URLSession = {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpCookieStorage = nil
        configuration.httpShouldSetCookies = false
        configuration.urlCredentialStorage = nil
        configuration.urlCache = nil
        configuration.requestCachePolicy = .reloadIgnoringLocalAndRemoteCacheData
        configuration.timeoutIntervalForRequest = 20
        configuration.timeoutIntervalForResource = 120
        configuration.httpMaximumConnectionsPerHost = 8
        return URLSession(configuration: configuration, delegate: HTTPPolicy(), delegateQueue: nil)
    }()

    static let installID: String = {
        let key = "kinosail.sources.device-id"
        if let saved = UserDefaults.standard.string(forKey: key), saved.range(of: "^[a-f0-9]{32}$", options: .regularExpression) != nil { return saved }
        let created = UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased()
        UserDefaults.standard.set(created, forKey: key)
        return created
    }()

    static var deviceName: String {
        #if os(tvOS)
        "Apple TV"
        #else
        UIDevice.current.userInterfaceIdiom == .pad ? "iPad" : "iPhone"
        #endif
    }

    static var version: String {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "1.0"
    }

    static func data(_ request: URLRequest, maximum: Int = 32 * 1024 * 1024, expected: Set<Int> = [200, 204]) async throws -> Data {
        do {
            let (data, _) = try await BoundedHTTPResponse.receive(request, session: session, maximum: maximum, expected: expected)
            return data
        } catch let error as ClientError { throw error }
        catch {
            if Task.isCancelled || (error as? URLError)?.code == .cancelled { throw CancellationError() }
            throw ClientError.unavailable
        }
    }

    static func json(_ request: URLRequest, maximum: Int = 32 * 1024 * 1024) async throws -> [String: Any] {
        let data = try await data(request, maximum: maximum, expected: [200])
        guard let value = try JSONSerialization.jsonObject(with: data) as? [String: Any] else { throw ClientError.invalidResponse }
        return value
    }

    static func url(_ base: URL, _ path: String, _ query: [String: String] = [:]) throws -> URL {
        guard var parts = URLComponents(url: base, resolvingAgainstBaseURL: false) else { throw ClientError.invalidResponse }
        parts.path = parts.path + path
        if !query.isEmpty { parts.queryItems = query.sorted { $0.key < $1.key }.map { URLQueryItem(name: $0.key, value: $0.value) } }
        guard let url = parts.url else { throw ClientError.invalidResponse }
        return url
    }

    static func identifier(_ value: Any?) -> String? {
        if let text = value as? String, !text.isEmpty, text.utf8.count <= 128,
           text.range(of: "^[A-Za-z0-9._:/-]+$", options: .regularExpression) != nil { return text }
        if let number = value as? NSNumber { return number.stringValue }
        return nil
    }

    static func text(_ value: Any?, max: Int = 512) -> String {
        guard let text = value as? String else { return "" }
        let cleaned = text.replacingOccurrences(of: "\u{200B}", with: "").trimmingCharacters(in: .whitespacesAndNewlines)
        return String(cleaned.prefix(max))
    }

    static func date(_ value: Any?) -> Date? {
        guard let text = value as? String, text.utf8.count <= 40 else { return nil }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let date = formatter.date(from: text) { return date }
        formatter.formatOptions = [.withInternetDateTime]
        if let date = formatter.date(from: text) { return date }
        let trimmed = text.replacingOccurrences(of: #"\.\d+"#, with: "", options: .regularExpression)
        return formatter.date(from: trimmed.hasSuffix("Z") ? trimmed : trimmed + "Z")
    }
}
