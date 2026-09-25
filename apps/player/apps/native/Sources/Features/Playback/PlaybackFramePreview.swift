#if os(iOS) || os(tvOS)
import AVFoundation
import SwiftUI

enum PlaybackFramePreview {
    @MainActor static func load(template: String?, client: ServerClient?, asset: AVAsset?, second: Int) async throws -> UIImage {
        guard (0...43200).contains(second), second.isMultiple(of: 10) else { throw ClientError.invalidResponse }
        if let template, let client {
            let (data, type) = try await client.resource(template.replacingOccurrences(of: "{second}", with: String(second)), maximum: 512 * 1024)
            guard type == "image/jpeg", let image = UIImage(data: data), image.size.width > 0, image.size.height > 0,
                  image.size.width <= 1024, image.size.height <= 1024,
                  image.size.width * image.size.height <= 1_048_576 else { throw ClientError.invalidResponse }
            return image
        }
        guard let file = (asset as? AVURLAsset)?.url, file.isFileURL else { throw ClientError.invalidResponse }
        let generator = AVAssetImageGenerator(asset: AVURLAsset(url: file))
        generator.appliesPreferredTrackTransform = true
        generator.maximumSize = CGSize(width: 320, height: 180)
        let (image, _) = try await generator.image(at: CMTime(seconds: Double(second), preferredTimescale: 600))
        return UIImage(cgImage: image)
    }
}
#endif
