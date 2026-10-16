#if os(tvOS)
import AVFoundation
import UIKit

enum PlaybackExternalMetadata {
    static func items(for item: MediaItem, artwork: Data? = nil) -> [AVMetadataItem] {
        var values: [(AVMetadataIdentifier, NSCopying & NSObjectProtocol)] = [(.commonIdentifierTitle, item.title as NSString)]
        if !subtitle(for: item).isEmpty { values.append((.iTunesMetadataTrackSubTitle, subtitle(for: item) as NSString)) }
        if !item.plot.isEmpty { values.append((.commonIdentifierDescription, item.plot as NSString)) }
        if let artwork { values.append((.commonIdentifierArtwork, artwork as NSData)) }
        return values.map { identifier, value in
            let metadata = AVMutableMetadataItem()
            metadata.identifier = identifier
            metadata.value = value
            metadata.extendedLanguageTag = "und"
            if identifier == .commonIdentifierArtwork { metadata.dataType = kCMMetadataBaseDataType_JPEG as String }
            return metadata.copy() as! AVMetadataItem
        }
    }

    static func subtitle(for item: MediaItem) -> String {
        if !item.show.isEmpty {
            let episode = item.season > 0 && item.episode > 0 ? "S\(item.season), E\(item.episode)" : ""
            return [item.show, episode].filter { !$0.isEmpty }.joined(separator: " · ")
        }
        return item.artist
    }

    static func artworkPath(for item: MediaItem) -> String {
        item.show.isEmpty ? item.poster : item.landscapeArtwork
    }

    static func jpeg(_ image: CGImage) -> Data? {
        UIImage(cgImage: image).jpegData(compressionQuality: 0.85)
    }
}

extension PlaybackEngine {
    func publishArtwork(for item: MediaItem, on nativeItem: AVPlayerItem) {
        let path = PlaybackExternalMetadata.artworkPath(for: item)
        guard !path.isEmpty, let client else { return }
        Task { [weak self, weak nativeItem] in
            guard let image = try? await self?.artwork.image(path: path, client: client, dimension: 800),
                  let data = PlaybackExternalMetadata.jpeg(image), let nativeItem else { return }
            nativeItem.externalMetadata = PlaybackExternalMetadata.items(for: item, artwork: data)
        }
    }
}
#endif
