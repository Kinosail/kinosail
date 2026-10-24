#if os(iOS)
import SwiftUI

extension OfflineDownload {
    var inProgress: Bool { ![.ready, .failed, .paused].contains(state) }
    var fraction: Double? {
        if state == .ready { return 1 }
        if totalBytes > 0 { return min(1, Double(receivedBytes) / Double(totalBytes)) }
        if state == .preparing, let preparingPercent { return Double(preparingPercent) / 100 }
        return nil
    }
    var storedBytes: Int64 { state == .ready ? max(totalBytes, receivedBytes) : receivedBytes }
    var statusTitle: String {
        switch state {
        case .ready: String(localized: "Downloaded")
        case .preparing: String(localized: "Preparing")
        case .queued: String(localized: "Queued")
        case .waiting: String(localized: "Waiting")
        case .verifying: String(localized: "Checking")
        case .paused: String(localized: "Paused")
        case .failed: String(localized: "Needs attention")
        case .downloading: fraction.map { "\(Int($0 * 100))%" } ?? String(localized: "Downloading")
        }
    }
}

extension Int64 {
    var downloadSize: String {
        let formatter = ByteCountFormatter()
        formatter.countStyle = .file
        formatter.allowsNonnumericFormatting = false
        return formatter.string(fromByteCount: self)
    }
}

extension AppSession {
    func download(for item: MediaItem) -> OfflineDownload? {
        let matches = downloads.downloads.filter { $0.item.id == item.id }
        return matches.first { $0.state == .ready } ?? matches.first
    }
}

struct DownloadRing: View {
    let download: OfflineDownload
    var size: CGFloat = 22
    var glyph: String?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        ZStack {
            Circle().stroke(KinoTheme.muted.opacity(0.35), lineWidth: size / 9)
            if download.state == .ready {
                Image(systemName: "checkmark.circle.fill")
                    .resizable().scaledToFit()
                    .foregroundStyle(KinoTheme.signal)
                    .transition(.scale.combined(with: .opacity))
            } else if let fraction = download.fraction {
                Circle().trim(from: 0, to: max(fraction, 0.02))
                    .stroke(KinoTheme.signal, style: StrokeStyle(lineWidth: size / 9, lineCap: .round))
                    .rotationEffect(.degrees(-90))
                    .animation(reduceMotion ? nil : .easeOut(duration: 0.4), value: fraction)
            } else if download.inProgress {
                PhaseAnimator([0.0, 360.0]) { angle in
                    Circle().trim(from: 0, to: 0.28)
                        .stroke(KinoTheme.signal, style: StrokeStyle(lineWidth: size / 9, lineCap: .round))
                        .rotationEffect(.degrees(reduceMotion ? -90 : angle))
                } animation: { _ in reduceMotion ? nil : .linear(duration: 1) }
            }
            if let glyph = glyph ?? (download.state == .failed ? "exclamationmark" : download.state == .paused ? "pause.fill" : nil) {
                Image(systemName: glyph)
                    .font(.system(size: size * 0.36, weight: .bold))
                    .foregroundStyle(KinoTheme.text)
                    .contentTransition(.symbolEffect(.replace))
            }
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}

struct DetailDownloadButton: View {
    let item: MediaItem
    let start: () -> Void
    @Environment(AppSession.self) private var session

    var body: some View {
        let download = session.download(for: item)
        Group {
            if let download {
                NavigationLink(value: ScreenDestination.downloads) {
                    Label {
                        Text(download.statusTitle).monospacedDigit().contentTransition(.numericText())
                    } icon: { DownloadRing(download: download, size: 20) }
                }
                .accessibilityLabel(download.state == .ready ? "Downloaded" : "Download \(download.statusTitle)")
                .accessibilityHint("Opens Downloads")
            } else {
                Button(action: start) { Label("Download", systemImage: "arrow.down") }
            }
        }
        .buttonStyle(SpotlightSecondaryButtonStyle())
        .animation(.snappy, value: download?.state)
        .sensoryFeedback(.success, trigger: download?.state == .ready) { old, new in !old && new }
    }
}

private struct DownloadBadge: ViewModifier {
    let item: MediaItem
    @Environment(AppSession.self) private var session

    func body(content: Content) -> some View {
        content.overlay(alignment: .topLeading) {
            if session.download(for: item)?.state == .ready {
                Image(systemName: "arrow.down.circle.fill")
                    .font(.system(size: 18, weight: .semibold))
                    .symbolRenderingMode(.palette)
                    .foregroundStyle(KinoTheme.signalInk, KinoTheme.signal)
                    .shadow(color: .black.opacity(0.4), radius: 4)
                    .padding(8)
                    .accessibilityLabel("Downloaded")
            }
        }
    }
}

extension View {
    func downloadBadge(_ item: MediaItem) -> some View { modifier(DownloadBadge(item: item)) }
}

struct DownloadStorageSummary: View {
    let downloads: [OfflineDownload]
    @State private var available: Int64?

    var body: some View {
        let used = downloads.reduce(Int64(0)) { $0 + $1.storedBytes }
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline) {
                Text(bytes(used)).font(.title2.bold()).monospacedDigit()
                Text("on this device").font(.subheadline).foregroundStyle(KinoTheme.muted)
                Spacer()
                if let available {
                    Text("\(bytes(available)) free").font(.subheadline).foregroundStyle(KinoTheme.muted).monospacedDigit()
                }
            }
            if let available, used + available > 0 {
                GeometryReader { geometry in
                    let share = Double(used) / Double(used + available)
                    ZStack(alignment: .leading) {
                        Capsule().fill(KinoTheme.raised)
                        Capsule().fill(KinoTheme.signal).frame(width: max(6, geometry.size.width * share))
                    }
                }
                .frame(height: 6)
                .accessibilityHidden(true)
            }
        }
        .padding(.vertical, 6)
        .accessibilityElement(children: .combine)
        .task { available = Self.availableCapacity() }
    }

    private func bytes(_ value: Int64) -> String { value.downloadSize }

    private static func availableCapacity() -> Int64? {
        let home = URL(fileURLWithPath: NSHomeDirectory())
        let values = try? home.resourceValues(forKeys: [.volumeAvailableCapacityForImportantUsageKey])
        return values?.volumeAvailableCapacityForImportantUsage
    }
}
#endif
