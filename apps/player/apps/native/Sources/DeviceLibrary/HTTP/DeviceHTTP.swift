import Foundation

struct DeviceRequest: Sendable {
    let method: String
    let path: String
    let query: [String: String]
    let headers: [String: String]
    let body: Data

    func json() throws -> JSONValue {
        guard !body.isEmpty else { throw DeviceFailure.invalid }
        return try StrictJSON.decode(body, maximum: 128 * 1024)
    }

    func number(_ name: String, fallback: Int, range: ClosedRange<Int>) throws -> Int {
        guard let raw = query[name] else { return fallback }
        guard let value = Int(raw), range.contains(value) else { throw DeviceFailure.invalid }
        return value
    }

    static func parse(head: String, body: Data, port: UInt16) throws -> DeviceRequest {
        let lines = head.components(separatedBy: "\r\n")
        let first = (lines.first ?? "").split(separator: " ", omittingEmptySubsequences: false)
        guard first.count == 3, ["GET", "HEAD", "POST", "PUT", "DELETE"].contains(String(first[0])),
              ["HTTP/1.1", "HTTP/1.0"].contains(String(first[2])) else { throw DeviceFailure.invalid }
        var headers: [String: String] = [:]
        for line in lines.dropFirst() where !line.isEmpty {
            guard let colon = line.firstIndex(of: ":") else { throw DeviceFailure.invalid }
            let name = String(line[..<colon]).lowercased()
            let value = line[line.index(after: colon)...].trimmingCharacters(in: .whitespaces)
            guard name.range(of: "^[a-z0-9-]{1,64}$", options: .regularExpression) != nil, headers[name] == nil,
                  name != "transfer-encoding", value.utf8.count <= 4096,
                  !value.unicodeScalars.contains(where: { CharacterSet.controlCharacters.contains($0) }) else { throw DeviceFailure.invalid }
            headers[name] = value
        }
        guard headers["host"] == "127.0.0.1:\(port)" else { throw DeviceFailure.invalid }
        let target = String(first[1])
        guard target.hasPrefix("/"), target.utf8.count <= 8192,
              let components = URLComponents(string: "http://127.0.0.1" + target) else { throw DeviceFailure.invalid }
        let path = components.path
        guard !path.split(separator: "/", omittingEmptySubsequences: false).contains(where: { $0 == "." || $0 == ".." }) else { throw DeviceFailure.invalid }
        var query: [String: String] = [:]
        for item in components.queryItems ?? [] {
            guard query[item.name] == nil, item.name.utf8.count <= 64, (item.value ?? "").utf8.count <= 2048 else { throw DeviceFailure.invalid }
            query[item.name] = item.value ?? ""
        }
        return DeviceRequest(method: String(first[0]), path: path, query: query, headers: headers, body: body)
    }
}

enum DeviceResponse: Sendable {
    case json(Int, JSONValue)
    case empty(Int)
    case data(Int, Data, String)
    case proxy(URLRequest, DeviceManifestRewrite?)

    static func failure(_ error: Error) -> DeviceResponse {
        let status: Int
        switch error {
        case DeviceFailure.invalid: status = 400
        case DeviceFailure.unauthorized: status = 401
        case DeviceFailure.notFound: status = 404
        case DeviceFailure.conflict: status = 409
        case DeviceFailure.unsupported: status = 501
        case ClientError.http(let code): status = code == 401 || code == 403 ? 502 : code >= 500 ? 502 : code
        default: status = 502
        }
        let message: String
        switch status {
        case 400: message = "The request is invalid."
        case 401: message = "Sign in again."
        case 404: message = "This title is not available."
        case 501: message = "This source does not support this feature."
        default: message = "The source could not be reached."
        }
        return .json(status, .object(["error": .string(message)]))
    }
}

struct DeviceManifestRewrite: Sendable {
    let itemID: String
    let base: URL
    let register: @Sendable (URL) throws -> String
}

enum DeviceFailure: Error, Equatable {
    case invalid, unauthorized, notFound, conflict, unsupported
}

protocol DeviceRequestHandler: Sendable {
    func handle(_ request: DeviceRequest) async -> DeviceResponse
}
