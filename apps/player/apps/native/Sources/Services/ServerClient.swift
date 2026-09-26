import Foundation
import CryptoKit
import OSLog

enum HTTPMethod: String, Sendable { case get = "GET", post = "POST", put = "PUT", delete = "DELETE" }

func diagnosticOperation(_ rawPath: String) -> String {
    guard rawPath.utf8.count <= 2048 else { return "other" }
    let parts = rawPath.prefix(while: { $0 != "?" }).split(separator: "/")
    guard let first = parts.first else { return "other" }
    if parts.count >= 3, first == "api", parts[1] == "v1" {
        if parts[2] == "items", parts.count >= 5,
           ["playback", "playback-preferences", "progress", "list", "reader"].contains(parts[4]) {
            return "items-\(parts[4])"
        }
        if ["items", "library", "me", "session", "quick-connect", "shows", "albums", "collections",
            "books", "cast", "remote-players", "downloads"].contains(parts[2]) { return String(parts[2]) }
        return "api-other"
    }
    return ["hls", "media", "art"].contains(first) ? String(first) : "other"
}

struct APIResponse: Sendable {
    let status: Int
    let body: JSONValue
}

/// The single owner of authenticated Server HTTP access.
actor ServerClient {
    let identity = UUID()
    let server: ServerAddress
    private let token: String
    private let session: URLSession
    private let networkLog = Logger(subsystem: "com.kinosail.player", category: "network")
    private var closed = false
    private(set) var viewerID: String?
    private var serverID: String?
    private(set) var associatedViewer: Viewer?
    private var mediaCache: LocalMediaCache?
    private var cacheDisabled = false
    private let cacheDirectory: URL?
    var catalogGeneration = UUID()
    var catalogRequests: [String: CatalogRequest] = [:]
    var reachability = ServerReachability.unknown
    var connectionObserver: (id: UUID, continuation: AsyncStream<ServerReachability>.Continuation)?
    var connectionSequence: UInt64 = 0
    private var requestSequence: UInt64 = 0

    init(server: ServerAddress, token: String = "", viewer: Viewer? = nil, protocolClasses: [AnyClass]? = nil, cacheDirectory: URL? = nil) throws {
        self.server = server
        self.token = token.isEmpty ? "" : try Input.secret(token)
        viewerID = viewer?.id
        serverID = viewer?.serverID
        associatedViewer = viewer
        self.cacheDirectory = cacheDirectory
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpCookieStorage = nil
        configuration.httpShouldSetCookies = false
        configuration.urlCredentialStorage = nil
        configuration.urlCache = nil
        configuration.requestCachePolicy = .reloadIgnoringLocalAndRemoteCacheData
        configuration.timeoutIntervalForRequest = 20
        configuration.timeoutIntervalForResource = 60
        configuration.httpMaximumConnectionsPerHost = 4
        configuration.protocolClasses = protocolClasses
        session = URLSession(configuration: configuration, delegate: HTTPPolicy(), delegateQueue: nil)
    }

    func request(_ path: String, method: HTTPMethod = .get, body: JSONValue? = nil, expected: Set<Int> = [200]) async throws -> APIResponse {
        guard path.hasPrefix("/api/v1/"), !expected.isEmpty, expected.count <= 8,
              expected.allSatisfy({ (200...499).contains($0) }), method != .get || body == nil else {
            throw ClientError.invalidInput("The Server request is invalid.")
        }
        var request = try authorizedRequest(path, method: method)
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let body {
            let data = try JSONEncoder().encode(body)
            guard data.count <= 128 * 1024 else { throw ClientError.invalidInput("The request is too large.") }
            _ = try StrictJSON.decode(data, maximum: 128 * 1024)
            request.httpBody = data
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        let (data, response) = try await receive(request, maximum: 2 * 1024 * 1024, expected: expected)
        if response.statusCode == 204 {
            guard data.isEmpty else { throw ClientError.invalidResponse }
            return APIResponse(status: response.statusCode, body: .null)
        }
        guard !data.isEmpty, response.mimeType?.lowercased() == "application/json" else { throw ClientError.invalidResponse }
        return APIResponse(status: response.statusCode, body: try StrictJSON.decode(data))
    }

    func authorizedRequest(_ path: String, method: HTTPMethod = .get) throws -> URLRequest {
        guard !closed else { throw CancellationError() }
        let url: URL
        let collectionPrefix = "/api/v1/collections/"
        if method == .get, path.hasPrefix(collectionPrefix) {
            // Metadata collection names can contain slashes. Keep them in one canonical URL segment.
            guard let name = String(path.dropFirst(collectionPrefix.count)).removingPercentEncoding,
                  path == collectionPrefix + Input.segment(try Input.collection(name)),
                  let collectionURL = URL(string: path, relativeTo: server.url)?.absoluteURL else { throw ClientError.invalidResponse }
            url = collectionURL
        } else { url = try server.mediaURL(path) }
        var request = URLRequest(url: url)
        request.httpMethod = method.rawValue
        request.httpShouldHandleCookies = false
        if !token.isEmpty { request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization") }
        if let viewerID { request.setValue(viewerID, forHTTPHeaderField: "X-Kinosail-Viewer-Profile") }
        return request
    }

    func associate(_ viewer: Viewer) throws {
        guard viewerID == nil || viewerID == viewer.id, serverID == nil || serverID == viewer.serverID else {
            throw ClientError.invalidInput("The Server or Viewer Profile changed. Connect again to continue.")
        }
        viewerID = viewer.id
        serverID = viewer.serverID
        associatedViewer = viewer
    }

    func resource(_ path: String, maximum: Int) async throws -> (Data, String) {
        guard (1...64 * 1024 * 1024).contains(maximum) else { throw ClientError.invalidInput("The resource limit is invalid.") }
        let (data, response) = try await receive(authorizedRequest(path), maximum: maximum)
        return (data, response.mimeType ?? "application/octet-stream")
    }

    private func receive(_ request: URLRequest, maximum: Int, expected: Set<Int> = [200]) async throws -> (Data, HTTPURLResponse) {
        try Task.checkCancellation()
        var request = request
        let requestID = UUID().uuidString
        request.setValue(requestID, forHTTPHeaderField: "X-Request-ID")
        let method = request.httpMethod ?? "GET"
        let operation = diagnosticOperation(request.url?.path ?? "")
        requestSequence &+= 1
        let sequence = requestSequence
        do {
            let (data, http) = try await BoundedHTTPResponse.receive(request, session: session, maximum: maximum, expected: expected)
            try Task.checkCancellation()
            guard !closed else { throw CancellationError() }
            recordConnection(.reachable, sequence: sequence)
            return (data, http)
        } catch is CancellationError { throw CancellationError() }
        catch let error as ClientError {
            if case .http(let status) = error {
                recordConnection(status >= 500 ? .unreachable : .reachable, sequence: sequence)
                if status >= 500 {
                    networkLog.error("HTTP request failed request_id=\(requestID, privacy: .public) operation=\(operation, privacy: .public) method=\(method, privacy: .public) status=\(status)")
                } else {
                    networkLog.warning("HTTP request failed request_id=\(requestID, privacy: .public) operation=\(operation, privacy: .public) method=\(method, privacy: .public) status=\(status)")
                }
            } else {
                networkLog.error("Invalid HTTP response request_id=\(requestID, privacy: .public) operation=\(operation, privacy: .public) method=\(method, privacy: .public)")
            }
            throw error
        }
        catch {
            if Task.isCancelled || (error as? URLError)?.code == .cancelled { throw CancellationError() }
            recordConnection(.unreachable, sequence: sequence)
            let code = (error as? URLError)?.errorCode ?? 0
            networkLog.warning("HTTP transport failed request_id=\(requestID, privacy: .public) operation=\(operation, privacy: .public) method=\(method, privacy: .public) code=\(code)")
            throw ClientError.unavailable
        }
    }

    func downloadAuthorization() throws -> DownloadAuthorization {
        guard !closed, let viewerID, let serverID else { throw ClientError.http(401) }
        return try DownloadAuthorization(server: server, serverID: serverID, profileID: viewerID, token: token)
    }

    func profileScope() throws -> String {
        guard !closed, let viewerID, let serverID else { throw ClientError.http(401) }
        let raw = "\(server.url.absoluteString)\n\(serverID)\n\(viewerID)"
        return SHA256.hash(data: Data(raw.utf8)).map { String(format: "%02x", $0) }.joined()
    }

    func cacheStore() throws -> LocalMediaCache? {
        guard !closed else { throw CancellationError() }
        guard !cacheDisabled, viewerID != nil, serverID != nil else { return nil }
        if mediaCache == nil { mediaCache = try LocalMediaCache(scope: profileScope(), directory: cacheDirectory) }
        return mediaCache
    }

    func discardMediaCache() async {
        let cache = try? cacheStore()
        cacheDisabled = true
        catalogGeneration = UUID()
        for request in catalogRequests.values { request.task.cancel() }
        catalogRequests = [:]
        await cache?.close(purge: true)
    }

    func close(purgeCache: Bool = false) async {
        if purgeCache { await discardMediaCache() }
        closed = true
        connectionObserver?.continuation.finish()
        connectionObserver = nil
        catalogGeneration = UUID()
        for request in catalogRequests.values { request.task.cancel() }
        catalogRequests = [:]
        session.invalidateAndCancel()
        await mediaCache?.close(purge: false)
    }

    deinit { session.invalidateAndCancel() }
}

/// Stateless delegate: redirects cannot carry authenticated requests elsewhere.
final class HTTPPolicy: NSObject, URLSessionTaskDelegate, Sendable {
    func urlSession(_ session: URLSession, task: URLSessionTask,
                    willPerformHTTPRedirection response: HTTPURLResponse, newRequest request: URLRequest,
                    completionHandler: @escaping @Sendable (URLRequest?) -> Void) {
        completionHandler(nil)
    }
}
