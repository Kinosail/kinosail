#if os(iOS)
  import CryptoKit
  import Foundation

  func decodeOffline<T: Decodable>(_ type: T.Type, from data: Data) throws -> T {
    guard data.count <= 2 * 1024 * 1024,
          let fields = try JSONSerialization.jsonObject(with: data) as? [String: Any] else { throw VerifiedDownload.failure("Invalid download metadata.") }
    let required: Set<String>
    var optional: Set<String> = []
    if type == DownloadManifest.self {
      required = ["version", "id", "size", "sha256", "chunkSize", "chunks"]
    } else if type == DownloadPreparation.self {
      required = ["scope", "key", "uri", "kind", "wifiOnly", "quota", "status", "error"]
      optional = ["retryAt"]
    } else {
      required = ["scope", "key", "uri", "manifest", "kind", "wifiOnly", "verified", "status", "error", "retries"]
      optional = ["retryAt"]
      guard let manifest = fields["manifest"] as? [String: Any] else { throw VerifiedDownload.failure("Invalid download manifest.") }
      let _: DownloadManifest = try decodeOffline(DownloadManifest.self, from: JSONSerialization.data(withJSONObject: manifest))
    }
    let keys = Set(fields.keys)
    guard required.isSubset(of: keys), keys.isSubset(of: required.union(optional)) else { throw VerifiedDownload.failure("Unknown download metadata.") }
    return try JSONDecoder().decode(type, from: data)
  }

  struct DownloadManifest: Codable, Equatable {
    let version: Int
    let id: String
    let size: Int64
    let sha256: String
    let chunkSize: Int64
    let chunks: [String]
    static let blockSize: Int64 = 8 * 1024 * 1024
    func validate() throws {
      guard version == 1, id.range(of: "^[a-f0-9]{16}$", options: .regularExpression) != nil,
            size > 0, size <= Self.blockSize * 16384, chunkSize == Self.blockSize,
            Self.digest(sha256), chunks.count == Int((size + chunkSize - 1) / chunkSize), chunks.allSatisfy(Self.digest)
      else { throw VerifiedDownload.failure("The download manifest is invalid.") }
    }

    static func digest(_ value: String) -> Bool {
      value.range(of: "^[a-f0-9]{64}$", options: .regularExpression) != nil
    }

    func length(_ index: Int) -> Int64 {
      min(chunkSize, size - Int64(index) * chunkSize)
    }
  }

  struct VerifiedDownload: Codable {
    let scope: String
    let key: String
    let uri: String
    let manifest: DownloadManifest
    let kind: String
    var wifiOnly: Bool
    var verified: [Bool]
    var status = "queued"
    var error = ""
    var retryAt: Date?
    var retries = 0
    var id: String {
      scope + "/" + key
    }

    var bytes: Int64 {
      verified.enumerated().reduce(0) { $0 + ($1.element ? manifest.length($1.offset) : 0) }
    }

    static func failure(_ message: String) -> NSError {
      NSError(domain: "KinosailOffline", code: 1, userInfo: [NSLocalizedDescriptionKey: message])
    }

    func validate() throws {
      try manifest.validate()
      guard DownloadManifest.digest(scope), DownloadManifest.digest(key), uri.utf8.count <= 2048,
            let url = URL(string: uri), let host = url.host,
            url.scheme == "https" || (url.scheme == "http" && approvedLocalHost(host)),
            url.user == nil, url.password == nil, url.query == nil, url.fragment == nil,
            url.path == "/api/v1/downloads/\(manifest.id)/file",
            ["video", "audio", "audiobook"].contains(kind), verified.count == manifest.chunks.count,
            ["queued", "waiting", "paused", "downloading", "verifying", "complete"].contains(status),
            error.utf8.count <= 256, (0 ... 5).contains(retries), status != "complete" || verified.allSatisfy({ $0 })
      else { throw Self.failure("The saved download is invalid.") }
    }

    static func hash(_ bytes: Data) -> String {
      SHA256.hash(data: bytes).map { String(format: "%02x", $0) }.joined()
    }
  }

  struct DownloadPreparation: Codable {
    let scope: String
    let key: String
    let uri: String
    let kind: String
    let wifiOnly: Bool
    let quota: Int64
    var status: String
    var error: String
    var retryAt: Date?
    var id: String {
      scope + "/" + key
    }

    func validate() throws {
      guard DownloadManifest.digest(scope), DownloadManifest.digest(key), uri.utf8.count <= 2048,
            let url = URL(string: uri), let host = url.host, url.user == nil, url.password == nil, url.query == nil, url.fragment == nil,
            url.scheme == "https" || url.scheme == "http" && approvedLocalHost(host),
            url.path.range(of: "^/api/v1/downloads/[a-f0-9]{16}/file$", options: .regularExpression) != nil,
            ["video", "audio", "audiobook"].contains(kind), ["preparing", "paused"].contains(status), error.utf8.count <= 256,
            quota == 0 || (1_073_741_824 ... 9_007_199_254_740_991).contains(quota) else { throw VerifiedDownload.failure("Download preparation is invalid.") }
    }
  }
#endif
