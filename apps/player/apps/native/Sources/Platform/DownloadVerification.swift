#if os(iOS)
import CryptoKit
import Darwin
import Foundation
import Synchronization

/// A cached verification is usable only while the app-owned file is unchanged.
/// Nanosecond ctime catches same-size rewrites even when mtime is restored.
struct DownloadFileStamp: Equatable, Sendable {
    let device: Int32
    let inode: UInt64
    let size: Int64
    let modified: timespec
    let changed: timespec

    init(_ url: URL) throws {
        var value = stat()
        guard url.resolvingSymlinksInPath().standardizedFileURL == url.standardizedFileURL,
              lstat(url.path, &value) == 0, value.st_mode & S_IFMT == S_IFREG else { throw ClientError.invalidResponse }
        device = value.st_dev; inode = value.st_ino; size = value.st_size
        modified = value.st_mtimespec; changed = value.st_ctimespec
    }

    static func == (lhs: Self, rhs: Self) -> Bool {
        lhs.device == rhs.device && lhs.inode == rhs.inode && lhs.size == rhs.size &&
        lhs.modified.tv_sec == rhs.modified.tv_sec && lhs.modified.tv_nsec == rhs.modified.tv_nsec &&
        lhs.changed.tv_sec == rhs.changed.tv_sec && lhs.changed.tv_nsec == rhs.changed.tv_nsec
    }
}

final class DownloadVerification: Sendable {
    let cancelled = Mutex(false)
    func cancel() { cancelled.withLock { $0 = true } }

    func verify(_ input: VerifiedDownload, at path: URL) throws -> (VerifiedDownload, DownloadFileStamp) {
        let before = try DownloadFileStamp(path)
        guard before.size == input.manifest.size else { throw ClientError.invalidResponse }
        var job = input
        let file = try FileHandle(forReadingFrom: path)
        defer { try? file.close() }
        var whole = SHA256()
        for index in job.verified.indices {
            guard !cancelled.withLock({ $0 }) else { throw CancellationError() }
            let block = try file.read(upToCount: Int(job.manifest.length(index))) ?? Data()
            job.verified[index] = block.count == Int(job.manifest.length(index)) && VerifiedDownload.hash(block) == job.manifest.chunks[index]
            whole.update(data: block)
        }
        guard before == (try DownloadFileStamp(path)), !cancelled.withLock({ $0 }) else { throw CancellationError() }
        if !job.verified.allSatisfy({ $0 }) || whole.finalize().map({ String(format: "%02x", $0) }).joined() != job.manifest.sha256 {
            job.status = "paused"; job.error = "Part of the download is missing or damaged. Resume to download that part again."
        }
        return (job, before)
    }
}
#endif
