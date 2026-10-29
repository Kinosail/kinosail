#if os(iOS) || os(tvOS)
import AVFoundation
import SwiftUI

@MainActor enum PlaybackFramePreview {
    private static let frames: NSCache<NSString, UIImage> = {
        let cache = NSCache<NSString, UIImage>()
        cache.countLimit = 360
        return cache
    }()
    private static var loading: [String: Task<UIImage, Error>] = [:]
    private static var lastSecond: [String: Int] = [:]
    private static let concurrentLoads = 4

    static func load(template: String?, client: ServerClient?, asset: AVAsset?, second: Int) async throws -> UIImage {
        guard (0...43200).contains(second), second.isMultiple(of: 10),
              let source = source(template: template, client: client, asset: asset) else { throw ClientError.invalidResponse }
        let step = (lastSecond[source].map { second < $0 ? -10 : 10 }) ?? 10
        lastSecond[source] = second
        let key = source + "@" + String(second)
        let image = frames.object(forKey: key as NSString)
        let task = image == nil ? start(key, template: template, client: client, asset: asset, second: second) : nil
        for offset in [1, 2, 3, -1] {
            let next = second + offset * step
            guard (0...43200).contains(next), loading.count < concurrentLoads else { continue }
            let nextKey = source + "@" + String(next)
            if frames.object(forKey: nextKey as NSString) == nil {
                _ = start(nextKey, template: template, client: client, asset: asset, second: next)
            }
        }
        if let image { return image }
        guard let task else { throw ClientError.invalidResponse }
        return try await task.value
    }

    private static func source(template: String?, client: ServerClient?, asset: AVAsset?) -> String? {
        if let template, let client { return client.identity.uuidString + template }
        guard let file = (asset as? AVURLAsset)?.url, file.isFileURL else { return nil }
        return file.path
    }

    private static func start(_ key: String, template: String?, client: ServerClient?, asset: AVAsset?, second: Int) -> Task<UIImage, Error> {
        if let task = loading[key] { return task }
        let task = Task<UIImage, Error> {
            defer { loading[key] = nil }
            let image = try await fetch(template: template, client: client, asset: asset, second: second)
            frames.setObject(image, forKey: key as NSString)
            return image
        }
        loading[key] = task
        return task
    }

    private static func fetch(template: String?, client: ServerClient?, asset: AVAsset?, second: Int) async throws -> UIImage {
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
