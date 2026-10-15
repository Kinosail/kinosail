import Foundation
import ImageIO

extension ArtworkLoader {
    static func artworkStorageKey(url: URL, dimension: Int) -> String {
        guard dimension == 4096, url.path.hasPrefix("/backdrop/"),
              URLComponents(url: url, resolvingAgainstBaseURL: false).map(ServerClient.cacheableMediaURL) == true else {
            return url.absoluteString
        }
        return "backdrop-source-v2:\(url.absoluteString)"
    }

    static func legacyBackdropImage(after error: Error, url: URL, dimension: Int, store: LocalMediaCache?) async throws -> CGImage? {
        guard (error as? ClientError)?.permitsOfflineRestore == true,
              artworkStorageKey(url: url, dimension: dimension) != url.absoluteString,
              let saved = await store?.read(url.absoluteString, kind: .artwork), saved.fresh else { return nil }
        try Task.checkCancellation()
        let image = try? await decode(saved.data, dimension: dimension)
        try Task.checkCancellation()
        return image
    }
}
