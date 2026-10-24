#if os(iOS)
import SwiftUI

struct DownloadQueueSummary: View {
    let downloads: [OfflineDownload]
    let network: DownloadNetwork
    let wifiOnly: Bool
    let allowCellular: () -> Void

    var body: some View {
        let active = downloads.filter(\.inProgress)
        let received = active.reduce(Int64(0)) { $0 + $1.receivedBytes }
        let total = active.reduce(Int64(0)) { $0 + $1.totalBytes }
        let rate = active.compactMap(\.bytesPerSecond).reduce(0, +)
        VStack(alignment: .leading, spacing: 8) {
            Label(headline(active.count), systemImage: symbol)
                .font(.subheadline.weight(.semibold))
                .foregroundStyle(waiting ? KinoTheme.muted : KinoTheme.text)
                .contentTransition(.symbolEffect(.replace))
            if total > 0 {
                Text(detail(received: received, total: total, rate: rate))
                    .font(.caption).foregroundStyle(.secondary).monospacedDigit()
                    .contentTransition(.numericText())
                ProgressView(value: Double(min(received, total)), total: Double(total))
                    .tint(KinoTheme.signal)
                    .animation(.easeOut(duration: 0.4), value: received)
            }
            if network.online != false && wifiOnly && network.expensive {
                Button("Use cellular data for downloads", systemImage: "antenna.radiowaves.left.and.right", action: allowCellular)
                    .font(.subheadline.weight(.semibold))
            }
        }
        .padding(.vertical, 6)
        .accessibilityElement(children: .contain)
    }

    private var waiting: Bool { network.online == false || (wifiOnly && network.expensive) || !network.server }

    private var symbol: String {
        if network.online == false { return "wifi.slash" }
        if wifiOnly && network.expensive { return "wifi.exclamationmark" }
        return network.server ? "arrow.down.circle" : "exclamationmark.arrow.triangle.2.circlepath"
    }

    private func headline(_ count: Int) -> String {
        if network.online == false { return String(localized: "Offline. Downloads continue when you’re connected.") }
        if wifiOnly && network.expensive { return String(localized: "Waiting for Wi-Fi") }
        if !network.server { return String(localized: "Can’t reach your Server. Kinosail will keep trying.") }
        return count == 1 ? String(localized: "Downloading 1 title") : String(localized: "Downloading \(count) titles")
    }

    private func detail(received: Int64, total: Int64, rate: Double) -> String {
        var parts = ["\(received.downloadSize) of \(total.downloadSize)"]
        if rate > 0 {
            let minutes = Int((Double(total - received) / rate / 60).rounded(.up))
            if minutes < 1 { parts.append(String(localized: "under a minute left")) }
            else if minutes < 60 { parts.append(String(localized: "about \(minutes) min left")) }
            else { parts.append(String(localized: "about \(minutes / 60) h \(minutes % 60) min left")) }
        }
        return parts.joined(separator: " · ")
    }
}
#endif
