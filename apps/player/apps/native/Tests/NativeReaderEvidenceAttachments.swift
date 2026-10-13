#if os(iOS)
import Foundation
import Testing

extension NativePreferencesUXTests {
    // Keep fictional rendered states and bounded metrics in the uploaded result,
    // including when a failed journey unwinds and its simulator is removed.
    func readerAttach(_ folder: URL) {
        let names: Set<String> = [
            "localized-footer.png", "localization.json", "pending-font.json", "geometry.json",
            "pending-initial.png", "pending.png", "loaded.png", "empty.png", "failed.png",
            "failed-operation.json", "pending-before-save.png", "failed-after-save.png",
            "loaded-after-save.png", "concurrent-save.json"
        ]
        for name in names {
            let file = folder.appendingPathComponent(name)
            guard FileManager.default.fileExists(atPath: file.path) else { continue }
            let values = try? file.resourceValues(forKeys: [.fileSizeKey, .isRegularFileKey, .isSymbolicLinkKey])
            guard values?.isRegularFile == true, values?.isSymbolicLink != true,
                  let size = values?.fileSize, (1...10_485_760).contains(size),
                  let data = try? Data(contentsOf: file, options: .mappedIfSafe), data.count == size else {
                Issue.record("Reader artifact \(name) is unavailable or exceeds its regular-file bound.")
                continue
            }
            Attachment.record(data, named: name)
        }
    }
}
#endif
