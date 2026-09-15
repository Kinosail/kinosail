#if os(iOS)
import Foundation

struct VerifiedDownloadSnapshot: Sendable {
    let key: String
    let bytes: Int64
    let total: Int64
    let status: String
    let message: String
    let manifest: OfflineManifest?
}

enum OfflineResource {
    static func url(_ raw: String) throws -> URL {
        guard raw.utf8.count <= 2048, let url = URL(string: raw), var origin = URLComponents(url: url, resolvingAgainstBaseURL: false),
              origin.user == nil, origin.password == nil, origin.query == nil, origin.fragment == nil,
              url.path.range(of: "\\A/api/v1/downloads/[a-f0-9]{16}/file\\z", options: .regularExpression) != nil else { throw ClientError.invalidResponse }
        origin.path = ""
        guard let base = origin.string else { throw ClientError.invalidResponse }
        let server = try ServerAddress(base)
        return try server.mediaURL(raw)
    }
}
#endif
