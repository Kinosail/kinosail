#if os(iOS)
import Foundation

struct TransferRate {
    private var samples: [String: (date: Date, bytes: Int64, rate: Double)] = [:]

    mutating func update(_ key: String, bytes: Int64, now: Date) {
        guard let last = samples[key] else { samples[key] = (now, bytes, 0); return }
        let seconds = now.timeIntervalSince(last.date)
        guard bytes >= last.bytes else { samples[key] = (now, bytes, 0); return }
        guard seconds >= 1 else { return }
        let instant = Double(bytes - last.bytes) / seconds
        samples[key] = (now, bytes, last.rate == 0 ? instant : last.rate * 0.7 + instant * 0.3)
    }

    func rate(_ key: String) -> Double? { samples[key].flatMap { $0.rate > 0 ? $0.rate : nil } }
}

extension OfflineDownloadManager {
    func preparationPercents(_ snapshots: [VerifiedDownloadSnapshot]) async -> [String: Int] {
        guard let client, !playback else { return [:] }
        let pending = catalog.records.filter { record in snapshots.first { $0.key == record.key }?.status == "preparing" }.prefix(8)
        return await withTaskGroup(of: (String, Int)?.self) { group in
            for record in pending {
                group.addTask {
                    guard let job = try? await client.preparedDownload(id: record.jobID), job.state == .preparing else { return nil }
                    return (record.key, job.progress)
                }
            }
            var result: [String: Int] = [:]
            for await entry in group { if let entry { result[entry.0] = entry.1 } }
            return result
        }
    }

    var quota: Int64 { Int64(preferences.downloadLimitGiB) * 1_073_741_824 }
    func activeScope(id: String) throws -> String {
        _ = try Input.hex(id, count: 64)
        guard let scope, catalog.records.contains(where: { $0.key == id }) else { throw ClientError.invalidInput("The download is unavailable for this Viewer Profile.") }
        return scope
    }
    func check(_ attempt: UUID) throws { try Task.checkCancellation(); guard generation == attempt else { throw CancellationError() } }
}
#endif
