import Foundation
import ImageIO
import UniformTypeIdentifiers

extension ArtworkLoader {
    static var imageLimit: Int { 32 * 1024 * 1024 }

    static var deviceMemoryBudget: Int {
        #if os(tvOS)
        64 * 1024 * 1024
        #else
        imageLimit
        #endif
    }

    static var deviceEntryLimit: Int {
        #if os(tvOS)
        160
        #else
        96
        #endif
    }

    static func decodedThumbnail(_ data: Data, dimension: Int) throws -> CGImage {
        guard [400, 500, 800, 1600, 4096].contains(dimension), data.count <= imageLimit else { throw ClientError.invalidResponse }
        guard let source = CGImageSourceCreateWithData(data as CFData, [kCGImageSourceShouldCache: false] as CFDictionary),
              CGImageSourceGetCount(source) <= 256,
              let properties = CGImageSourceCopyPropertiesAtIndex(source, 0, nil) as? [CFString: Any],
              let width = properties[kCGImagePropertyPixelWidth] as? Int,
              let height = properties[kCGImagePropertyPixelHeight] as? Int,
              width > 0, height > 0, width <= 32_768, height <= 32_768, width * height <= 80_000_000,
              let image = CGImageSourceCreateThumbnailAtIndex(source, 0, [
                kCGImageSourceCreateThumbnailFromImageAlways: true,
                kCGImageSourceCreateThumbnailWithTransform: true,
                kCGImageSourceThumbnailMaxPixelSize: dimension,
                kCGImageSourceShouldCacheImmediately: true
              ] as CFDictionary) else { throw ClientError.invalidResponse }
        return image
    }

    static func decode(_ data: Data, dimension: Int) async throws -> CGImage {
        let work = Task.detached(priority: Task.currentPriority) {
            try Task.checkCancellation()
            return try decodedThumbnail(data, dimension: dimension)
        }
        return try await withTaskCancellationHandler {
            try await work.value
        } onCancel: {
            work.cancel()
        }
    }

    static func thumbnail(_ data: Data, dimension: Int) throws -> Data {
        let image = try decodedThumbnail(data, dimension: dimension)
        let output = NSMutableData()
        let alpha = ![CGImageAlphaInfo.none, .noneSkipFirst, .noneSkipLast].contains(image.alphaInfo)
        guard let destination = CGImageDestinationCreateWithData(output, (alpha ? UTType.png.identifier : UTType.jpeg.identifier) as CFString, 1, nil) else {
            throw ClientError.invalidResponse
        }
        CGImageDestinationAddImage(destination, image, [kCGImageDestinationLossyCompressionQuality: 0.88] as CFDictionary)
        guard CGImageDestinationFinalize(destination), output.length <= imageLimit else { throw ClientError.invalidResponse }
        return output as Data
    }
}
