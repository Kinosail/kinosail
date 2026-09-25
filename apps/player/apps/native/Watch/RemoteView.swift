import SwiftUI

private let electric = Color(red: 0.77, green: 1, blue: 0.28)

struct RemoteView: View {
    @Environment(WatchRemoteSession.self) private var remote
    @Environment(MovieHeartTracker.self) private var heart

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 14) {
                    if remote.players.count > 1 {
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
                        HStack(spacing: 10) {
                            Button(player.audio ? "Previous" : "Back 15 seconds", systemImage: player.audio ? "backward.end.fill" : "gobackward.15") {
                                Task { await remote.command(player.audio ? "previous" : "backward") }
                            }
                            .buttonStyle(.bordered).accessibilityLabel(player.audio ? "Previous track" : "Back 15 seconds")
                            Button(player.playing ? "Pause" : "Play", systemImage: player.playing ? "pause.fill" : "play.fill") {
                                Task { await remote.command(player.playing ? "pause" : "play") }
                            }
                            .buttonStyle(.borderedProminent).tint(electric).foregroundStyle(.black)
                            .accessibilityLabel(player.playing ? "Pause \(player.title)" : "Play \(player.title)")
                            Button(player.audio ? "Next" : "Forward 30 seconds", systemImage: player.audio ? "forward.end.fill" : "goforward.30") {
                                Task { await remote.command(player.audio ? "next" : "forward") }
                            }
                            .buttonStyle(.bordered).accessibilityLabel(player.audio ? "Next track" : "Forward 30 seconds")
                        }
                        .labelStyle(.iconOnly).font(.title3)
                        .disabled(remote.busy)
                        if player.duration > 0 {
                            NavigationLink { ScrubView() } label: { Label("Go to time", systemImage: "timeline.selection") }
                                .font(.caption).buttonStyle(.bordered)
                        }
                        if heart.isTracking && heart.matches(player) {
                            Button("Stop heart graph", systemImage: "stop.circle") { Task { await heart.stop() } }
                                .font(.caption).buttonStyle(.plain).foregroundStyle(.secondary)
                        } else if player.duration > 0 && !player.audio && !heart.isTracking {
                            Button("Start heart graph", systemImage: "heart") { Task { await heart.start(for: player) } }
                                .font(.caption).buttonStyle(.bordered)
                        }
                    } else {
                        ContentUnavailableView("Nothing playing", systemImage: "play.rectangle", description: Text("Start a title on your iPhone or Apple TV."))
                    }
                    if heart.timeline != nil {
                        NavigationLink { HeartGraphView() } label: { Label(heart.isTracking ? "Heart graph" : "Last heart graph", systemImage: "heart.text.square") }
                            .font(.caption).buttonStyle(.bordered)
                    }
                    if let message = remote.message ?? heart.message {
                        Text(message).font(.caption2).foregroundStyle(.secondary)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.horizontal, 8)
            }
            .navigationTitle("Now playing")
            .task {
                while !Task.isCancelled {
                    await remote.refresh()
                    if let id = heart.timeline?.targetID, let player = remote.players.first(where: { $0.id == id }) { heart.note(player) }
                    try? await Task.sleep(for: .seconds(5))
                }
            }
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
