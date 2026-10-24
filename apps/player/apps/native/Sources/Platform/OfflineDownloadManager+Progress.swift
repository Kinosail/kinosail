#if os(iOS)
import Foundation

struct TransferRate {
    private var samples: [String: (date: Date, bytes: Int64, rate: Double, changed: Date)] = [:]

    mutating func update(_ key: String, bytes: Int64, now: Date) {
        guard let last = samples[key] else { samples[key] = (now, bytes, 0, now); return }
        let seconds = now.timeIntervalSince(last.date)
        guard bytes >= last.bytes else { samples[key] = (now, bytes, 0, now); return }
        guard seconds >= 1 else { return }
        let instant = Double(bytes - last.bytes) / seconds
        samples[key] = (now, bytes, last.rate == 0 ? instant : last.rate * 0.7 + instant * 0.3, bytes > last.bytes ? now : last.changed)
    }

    func rate(_ key: String) -> Double? { samples[key].flatMap { $0.rate > 0 ? $0.rate : nil } }
    func stalled(_ key: String, now: Date) -> Bool { samples[key].map { now.timeIntervalSince($0.changed) >= 20 } ?? false }
}

struct DownloadNetwork: Equatable {
    var online: Bool?
    var expensive = false
    var server = true
}

extension OfflineDownloadManager {
    static let offlineWait = String(localized: "Waiting for a network connection. Kinosail continues when you’re back online.")
    static let wifiWait = String(localized: "Waiting for Wi-Fi. You can allow cellular data in Download settings.")
    static let stalledNote = String(localized: "The connection is slow. Kinosail keeps trying.")

    func updateNetwork(_ value: DownloadNetwork) async {
        let returned = (network.online == false && value.online != false) || (!network.server && value.server)
        network = value
        await refresh()
        if returned { await resumeInterrupted() }
    }

    func retryWaiting() async {
        guard let scope else { return }
        try? await engine.retryNow(scope)
        await refresh()
    }

    func refresh() async {
        guard let scope else { return }
        let attempt = generation
        do {
            let snapshots = try await engine.snapshot(scope)
            let percents = await preparationPercents(snapshots)
            let now = Date(); for snapshot in snapshots { rates.update(snapshot.key, bytes: snapshot.bytes, now: now) }
            try check(attempt)
            downloads = catalog.records.map { record in present(record, snapshot: snapshots.first { $0.key == record.key }, percents: percents, now: now) }
            message = nil
            if downloads.contains(where: \.stalled), now.timeIntervalSince(probed) >= 10, let client {
                probed = now
                Task { _ = try? await client.viewer() }
            }
        } catch is CancellationError {} catch { if generation == attempt { message = AppSession.message(error) } }
    }

    private func present(_ record: OfflineRecord, snapshot: VerifiedDownloadSnapshot?, percents: [String: Int], now: Date) -> OfflineDownload {
        var state: OfflineDownload.State
        if record.deleting { state = .failed }
        else if snapshot?.status == "complete" { state = .ready }
        else if playback, ["preparing", "queued", "waiting", "downloading"].contains(snapshot?.status ?? "queued") { state = .waiting }
        else { state = OfflineDownload.State(rawValue: snapshot?.status ?? (record.error.isEmpty ? "queued" : "failed")) ?? .failed }
        var explanation = record.deleting ? "Removal interrupted. Remove this download again." : playback && state == .waiting ? "Downloads resume after playback." : snapshot?.message ?? record.error
        let retryAt = snapshot?.retryAt.flatMap { $0 > now ? $0 : nil }
        let moving = [.downloading, .queued, .waiting].contains(state) && !playback
        if moving && network.online == false { state = .waiting; explanation = Self.offlineWait }
        else if moving && preferences.wifiOnly && network.expensive { state = .waiting; explanation = Self.wifiWait }
        else if moving && retryAt != nil { state = .waiting }
        else if moving && !network.server && rates.stalled(record.key, now: now) { state = .waiting; explanation = DownloadRecovery.unreachable }
        let stalled = state == .downloading && rates.stalled(record.key, now: now)
        if stalled && explanation.isEmpty { explanation = Self.stalledNote }
        return OfflineDownload(id: record.key, item: record.item, quality: record.quality, state: state, receivedBytes: snapshot?.bytes ?? 0,
                               totalBytes: snapshot?.total ?? 0, message: explanation.isEmpty ? nil : explanation, preparingSince: snapshot?.preparingSince,
                               preparingPercent: state == .preparing ? percents[record.key] : nil,
                               bytesPerSecond: state == .downloading && !stalled ? rates.rate(record.key) : nil,
                               retryAt: state == .waiting ? retryAt : nil, stalled: stalled)
    }

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
