#if os(iOS)
import SwiftUI

struct DownloadRow: View {
    let download: OfflineDownload
    let compact: Bool
    let offline: Bool
    let disabled: Bool
    let toggle: () -> Void
    let remove: () -> Void

    private var ready: Bool { download.state == .ready }
    private var canToggle: Bool { download.state != .verifying && !(offline && !download.inProgress) }

    var body: some View {
        Group {
            if ready {
                NavigationLink(value: ScreenDestination.offlinePlayback(download.id)) { content }
                    .accessibilityHint("Plays offline")
            } else { content }
        }
        .swipeActions(edge: .trailing) {
            Button("Remove", systemImage: "trash", role: .destructive, action: remove).disabled(disabled)
        }
        .swipeActions(edge: .leading) {
            if !ready && canToggle {
                Button(toggleTitle, systemImage: toggleSymbol, action: toggle).tint(KinoTheme.signal).disabled(disabled)
            }
        }
        .contextMenu {
            if ready {
                NavigationLink(value: ScreenDestination.offlinePlayback(download.id)) { Label("Play offline", systemImage: "play.fill") }
            } else if canToggle {
                Button(toggleTitle, systemImage: toggleSymbol, action: toggle).disabled(disabled)
            }
            Button("Remove download", systemImage: "trash", role: .destructive, action: remove).disabled(disabled)
        }
    }

    private var content: some View {
        HStack(spacing: 14) {
            if !compact { artwork }
            VStack(alignment: .leading, spacing: 4) {
                Text(download.item.title).font(.headline).lineLimit(compact ? nil : 2)
                if !subtitle.isEmpty { Text(subtitle).font(.caption).foregroundStyle(KinoTheme.muted).lineLimit(compact ? nil : 1) }
                status
                if let message = download.message { Text(message).font(.caption).foregroundStyle(.secondary) }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            if !ready {
                Button(action: toggle) { DownloadRing(download: download, size: 30, glyph: toggleSymbol) }
                    .buttonStyle(.plain)
                    .frame(width: 44, height: 44)
                    .contentShape(.rect)
                    .disabled(disabled || !canToggle)
                    .accessibilityLabel(toggleTitle)
                    .accessibilityValue(download.statusTitle)
            }
        }
        .padding(.vertical, 6)
        .accessibilityElement(children: .combine)
    }

    private var artwork: some View {
        Artwork(path: download.item.isAudio ? download.item.poster : download.item.landscapeArtwork, symbol: download.item.kind.symbol,
                ratio: download.item.isAudio ? 1 : 16 / 9, dimension: 800, fillsFrame: true)
            .frame(width: download.item.isAudio ? 68 : 120)
            .background(KinoTheme.surface)
            .overlay {
                if !ready { Color.black.opacity(0.35) }
            }
            .overlay(alignment: .bottomLeading) {
                if !ready, let fraction = download.fraction {
                    GeometryReader { geometry in
                        Capsule().fill(KinoTheme.signal).frame(width: geometry.size.width * fraction, height: 3)
                            .frame(maxHeight: .infinity, alignment: .bottom)
                    }
                    .animation(.easeOut(duration: 0.4), value: fraction)
                }
            }
            .clipShape(.rect(cornerRadius: 10))
            .accessibilityHidden(true)
    }

    @ViewBuilder private var status: some View {
        if ready {
            Label("\(bytes(download.storedBytes)) · \(download.quality.title)", systemImage: "checkmark.circle.fill")
                .font(.caption.weight(.medium)).foregroundStyle(KinoTheme.signal).monospacedDigit()
        } else if download.state == .preparing || (download.state == .queued && download.totalBytes == 0) {
            TimelineView(.periodic(from: .now, by: 1)) { context in
                Text(preparingDetail(now: context.date)).font(.caption).foregroundStyle(.secondary).monospacedDigit()
            }
        } else if let retryAt = download.retryAt {
            TimelineView(.periodic(from: .now, by: 1)) { context in
                Text(retryDetail(retryAt, now: context.date)).font(.caption).foregroundStyle(.secondary).monospacedDigit()
            }
        } else if download.totalBytes > 0 && download.state == .downloading {
            Text(transferDetail).font(.caption).foregroundStyle(.secondary).monospacedDigit()
        } else if download.totalBytes > 0 && download.inProgress {
            Text("\(download.statusTitle) · \(bytes(download.receivedBytes)) of \(bytes(download.totalBytes))").font(.caption).foregroundStyle(.secondary).monospacedDigit()
        } else {
            Text("\(download.statusTitle) · \(download.quality.title)").font(.caption).foregroundStyle(.secondary)
        }
    }

    private var subtitle: String {
        let item = download.item
        guard !item.show.isEmpty, item.season > 0 || item.episode > 0 else { return item.subtitle }
        return "\(item.show) · S\(item.season) E\(item.episode)"
    }
    private var toggleTitle: String {
        [.paused, .failed].contains(download.state) ? String(localized: "Resume download") : String(localized: "Pause download")
    }
    private var toggleSymbol: String {
        download.state == .failed ? "arrow.clockwise" : download.state == .paused ? "arrow.down" : "pause.fill"
    }
    private func bytes(_ value: Int64) -> String { value.downloadSize }
    private var transferDetail: String {
        var parts = ["\(bytes(download.receivedBytes)) of \(bytes(download.totalBytes))"]
        if let rate = download.bytesPerSecond, rate > 0 {
            parts.append("\(remaining(Double(download.totalBytes - download.receivedBytes) / rate)) left")
        }
        return parts.joined(separator: " · ")
    }
    private func retryDetail(_ date: Date, now: Date) -> String {
        let seconds = Int(date.timeIntervalSince(now).rounded(.up))
        let progress = download.totalBytes > 0 ? "\(bytes(download.receivedBytes)) of \(bytes(download.totalBytes)) · " : ""
        return seconds > 0 ? "\(progress)Trying again in \(seconds) s" : "\(progress)Trying again now"
    }
    private func remaining(_ seconds: Double) -> String {
        let minutes = Int((seconds / 60).rounded(.up))
        return minutes < 1 ? "under a minute" : minutes < 60 ? "\(minutes) min" : "\(minutes / 60) h \(minutes % 60) min"
    }
    private func preparingDetail(now: Date) -> String {
        let what = download.quality == .original ? "Packaging the original file" : "Making a \(download.quality.title.lowercased()) copy"
        let percent = download.preparingPercent.map { "\($0)% · " } ?? ""
        guard let since = download.preparingSince else { return "\(percent)\(what) on your Server" }
        let seconds = max(0, now.timeIntervalSince(since))
        if let done = download.preparingPercent, done >= 3 { return "\(percent)\(what) · about \(remaining(seconds * Double(100 - done) / Double(done))) left" }
        return "\(percent)\(what) · \(Int(seconds / 60)) min \(Int(seconds) % 60)s so far"
    }
}
#endif
