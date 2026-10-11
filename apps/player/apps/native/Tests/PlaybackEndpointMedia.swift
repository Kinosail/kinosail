#if os(tvOS)
import CryptoKit
import Foundation
import Testing

private final class PlaybackEndpointBundleMarker: NSObject {}

enum PlaybackEndpointMedia {
    enum Asset {
        case source, manifest, segment

        var metadata: (name: String, suffix: String, bytes: Int, sha256: String) {
            switch self {
            case .source:
                return ("endpoint-source", "mp4", 288629,
                        "fca8542e89dcbfc618e21fdbe98261caacc72070ab92866ac604783ba4c37ff6")
            case .manifest:
                return ("endpoint-index", "m3u8", 147,
                        "6e1aec2f21470e8ac0c5a994d7221f8f28aac34da1155ecf6cff3142f8b37be7")
            case .segment:
                return ("endpoint-segment-00", "bin", 313020,
                        "353c2f395d892f8efcd08be20a9577bfa31679a550a2fb01921626594c1b7a0d")
            }
        }
    }

    static func data(_ asset: Asset) throws -> Data {
        let metadata = asset.metadata
        let bundle = Bundle(for: PlaybackEndpointBundleMarker.self)
        let url = try #require(bundle.url(forResource: metadata.name, withExtension: metadata.suffix),
                               "Prerequisite: synthetic endpoint media must be copied into the XCTest bundle")
        let values = try url.resourceValues(forKeys: [.fileSizeKey, .isRegularFileKey, .isSymbolicLinkKey])
        try #require(values.isRegularFile == true && values.isSymbolicLink != true &&
                     values.fileSize == metadata.bytes && metadata.bytes <= 1024 * 1024,
                     "Prerequisite: synthetic endpoint asset has the expected bounded size")
        let data = try Data(contentsOf: url)
        let digest = SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
        try #require(digest == metadata.sha256, "Prerequisite: synthetic endpoint asset matches the qualified bytes")
        return data
    }
}
#endif
