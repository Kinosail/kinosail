import SwiftUI

private let electric = Color(red: 0.77, green: 1, blue: 0.28)

struct RemoteView: View {
    @Environment(WatchRemoteSession.self) private var remote
    @Environment(MovieHeartTracker.self) private var heart
    @State private var page = 0

    var body: some View {
        TabView(selection: $page) {
            NavigationStack { remotePage }
                .tag(0)
            if heart.timeline != nil {
                NavigationStack { HeartGraphView(isVisible: page == 1) }
                    .tag(1)
            }
        }
        .tabViewStyle(.verticalPage)
        .task {
            while !Task.isCancelled {
                await remote.refresh()
                if let id = heart.timeline?.targetID, let player = remote.players.first(where: { $0.id == id }) { heart.note(player) }
                try? await Task.sleep(for: .seconds(5))
            }
        }
    }

    private var remotePage: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 10) {
                if !remote.players.isEmpty && (remote.players.count > 1 || remote.selected == nil) {
                    NavigationLink { DevicePickerView() } label: {
                        Label(remote.selected?.device ?? "Choose player", systemImage: "hifispeaker.and.homepod")
                            .font(.caption.weight(.semibold))
                    }
                } else {
                    Text(remote.selected?.device ?? "Kinosail")
                        .font(.caption.weight(.semibold)).foregroundStyle(electric)
                }

                if let player = remote.selected, player.active {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(player.title).font(.title3.bold()).lineLimit(2)
                        if !player.subtitle.isEmpty { Text(player.subtitle).font(.caption).foregroundStyle(.secondary).lineLimit(1) }
                    }
                    if player.duration > 0 {
                        ProgressView(value: player.position, total: player.duration).tint(electric)
                            .accessibilityLabel("Playback position")
                        HStack {
                            Text(clock(player.position))
                            Spacer()
                            Text(clock(player.duration))
                        }.font(.caption2.monospacedDigit()).foregroundStyle(.secondary)
                    }
                    HStack(spacing: 8) {
                        Button {
                            Task { await remote.command(player.audio ? "previous" : "backward") }
                        } label: { Image(systemName: player.audio ? "backward.end.fill" : "gobackward.15")
                            .frame(maxWidth: .infinity, minHeight: 44) }
                        .buttonStyle(.bordered).accessibilityLabel(player.audio ? "Previous track" : "Back 15 seconds")
                        Button {
                            Task { await remote.command(player.playing ? "pause" : "play") }
                        } label: { Image(systemName: player.playing ? "pause.fill" : "play.fill")
                            .frame(maxWidth: .infinity, minHeight: 44) }
                        .buttonStyle(.borderedProminent).tint(electric).foregroundStyle(.black)
                        .accessibilityLabel(player.playing ? "Pause \(player.title)" : "Play \(player.title)")
                        Button {
                            Task { await remote.command(player.audio ? "next" : "forward") }
                        } label: { Image(systemName: player.audio ? "forward.end.fill" : "goforward.30")
                            .frame(maxWidth: .infinity, minHeight: 44) }
                        .buttonStyle(.bordered).accessibilityLabel(player.audio ? "Next track" : "Forward 30 seconds")
                    }
                    .font(.title3)
                    .disabled(remote.busy)
                    if player.duration > 0 {
                        NavigationLink { ScrubView() } label: { Label("Go to time", systemImage: "timeline.selection") }
                            .font(.caption).buttonStyle(.bordered)
                    }
                    if !heart.isTracking && player.duration > 0 && !player.audio {
                        Button("Start heart graph", systemImage: "heart") {
                            Task {
                                await heart.start(for: player)
                                if heart.isTracking && heart.matches(player) { page = 1 }
                            }
                        }
                        .font(.caption).buttonStyle(.bordered)
                    }
                } else if remote.selected == nil && remote.selectedID != nil {
                    Label("Selected player unavailable", systemImage: "tv.slash")
                        .font(.caption).foregroundStyle(.secondary)
                } else {
                    Label("Nothing playing", systemImage: "play.rectangle")
                        .font(.headline)
                    Text("Start a title on your iPhone or Apple TV.")
                        .font(.caption).foregroundStyle(.secondary)
                }
                if heart.timeline != nil {
                    Button { page = 1 } label: {
                        HStack {
                            Label(heart.isTracking ? "Heart graph" : "Last heart graph", systemImage: "heart.text.square")
                            Spacer()
                            Image(systemName: "chevron.up")
                        }
                        .font(.caption.weight(.semibold)).foregroundStyle(electric)
                        .padding(.vertical, 6)
                    }
                    .buttonStyle(.plain)
                    .accessibilityHint("Swipe up to open the Heart page")
                }
                if let message = heart.message ?? remote.message {
                    Text(message).font(.caption2).foregroundStyle(.secondary)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 8)
        }
    }
}

private struct DevicePickerView: View {
    @Environment(WatchRemoteSession.self) private var remote
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        List(remote.players) { player in
            Button {
                remote.select(player.id)
                dismiss()
            } label: {
                VStack(alignment: .leading, spacing: 3) {
                    Text(player.device).font(.headline)
                    Text(player.active ? player.title : "Nothing playing").font(.caption).foregroundStyle(.secondary).lineLimit(1)
                }
            }
        }
        .navigationTitle("Players")
    }
}

private struct ScrubView: View {
    @Environment(WatchRemoteSession.self) private var remote
    @State private var position = 0.0

    var body: some View {
        VStack(spacing: 16) {
            Text(clock(position)).font(.title2.monospacedDigit())
            if let player = remote.selected, player.duration > 0 {
                Slider(value: $position, in: 0...player.duration)
                    .accessibilityLabel("Playback position")
                Button("Go to time") { Task { await remote.command("seek", position: position) } }
                    .buttonStyle(.borderedProminent).tint(electric).foregroundStyle(.black)
                    .disabled(remote.busy)
            }
        }
        .navigationTitle("Go to time")
        .onAppear { position = remote.selected?.position ?? 0 }
    }
}

func clock(_ seconds: Double) -> String {
    let value = Int(max(0, seconds))
    return value >= 3600 ? String(format: "%d:%02d:%02d", value / 3600, value / 60 % 60, value % 60)
                         : String(format: "%d:%02d", value / 60, value % 60)
}
