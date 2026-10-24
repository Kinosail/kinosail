#if os(iOS)
import Foundation

extension OfflineDownloadManager {
    func resumeInterrupted() async {
        let interrupted: Set<String> = [VerifiedDownload.interrupted, VerifiedDownload.preparationInterrupted]
        for download in downloads where download.state == .paused && interrupted.contains(download.message ?? "") {
            do { try await resume(id: download.id) } catch { continue }
        }
    }
}
#endif
