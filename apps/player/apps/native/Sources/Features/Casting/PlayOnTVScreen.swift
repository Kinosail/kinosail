import AVKit
import SwiftUI

struct PlayOnTVScreen: View {
    let itemID: String
    @Environment(AppSession.self) private var session
    @State private var devices: [CastDevice] = []
    @State private var scanning = false
    @State private var scanned = false
    @State private var message: String?
    @State private var seekPosition = 0.0
    @State private var isAudio = false
    #if os(tvOS)
    @Namespace private var castFocus
    #endif

    var body: some View {
        List {
            #if os(iOS)
            Section("AirPlay") {
                HStack {
                    Text("Choose an AirPlay \(isAudio ? "speaker" : "TV")")
                    Spacer()
                    AudioRoutePicker(video: !isAudio).frame(width: 48, height: 48).accessibilityLabel("Choose AirPlay output")
                }
                Text("Choose a receiver, then play this title here. Screen Mirroring in Control Center also works with compatible TVs.").foregroundStyle(.secondary)
                NavigationLink("Play on this device", value: ScreenDestination.playback(itemID))
            }
            #endif
            if let cast = session.casting.session {
                Section(cast.deviceName ?? "DLNA receiver") {
                    Text(cast.title).font(.headline)
                    if let status = session.casting.status {
                        Text("\(status.state.rawValue.capitalized) · \(status.position.clock) / \(status.duration.clock)")
                        HStack {
                            Button("Play", systemImage: "play.fill") { perform { try await session.casting.command(.play) } }
                                #if os(tvOS)
                                .tvOSDefaultPlayFocus(in: castFocus, id: "cast.play.\(cast.id)")
                                #endif
                            Button("Pause", systemImage: "pause.fill") { perform { try await session.casting.command(.pause) } }
                            Button("Stop", systemImage: "stop.fill") { perform { try await session.casting.command(.stop) } }
                        }.disabled(session.casting.busy)
                        if cast.duration > 0 {
                            #if os(iOS)
                            Slider(value: $seekPosition, in: 0...cast.duration).accessibilityLabel("Receiver playback position")
                            Button("Seek to \(seekPosition.clock)") { perform { try await session.casting.command(.seek(seekPosition)) } }.disabled(session.casting.busy)
                            #else
                            Button("Back 30 seconds") { perform { try await session.casting.command(.seek(max(0, status.position - 30))) } }
                            Button("Forward 30 seconds") { perform { try await session.casting.command(.seek(min(cast.duration, status.position + 30))) } }
                            #endif
                        }
                    } else { Text("Checking the receiver…").foregroundStyle(.secondary) }
                    if let message = session.casting.message { Text(message).foregroundStyle(.secondary) }
                    Button("Disconnect receiver", role: .destructive) { perform { try await session.casting.disconnect() } }.disabled(session.casting.busy)
                    Text("Disconnecting removes the receiver’s access. Choose Stop first to end playback there.").font(.footnote).foregroundStyle(.secondary)
                }
            } else {
                Section("DLNA receivers") {
                    Button(scanning ? "Searching…" : "Find receivers") { scan() }.disabled(scanning || session.casting.busy)
                    Text("Your Server finds compatible speakers and TVs on its local network.").font(.footnote).foregroundStyle(.secondary)
                    if scanned && devices.isEmpty { Text("No receivers found. Make sure the speaker or TV is on and connected to the Server’s network.") }
                    ForEach(devices) { device in
                        Button { start(device) } label: { Label("\(device.name) · DLNA", systemImage: "tv") }.disabled(session.casting.busy)
                    }
                }
            }
            if let message { Section { Text(message).foregroundStyle(.secondary) } }
        }
        .tvOSConfigurationLayout(title: "Play on another device", symbol: "tv")
        .configurationNavigationTitle("Play on another device")
        #if os(tvOS)
        .focusScope(castFocus)
        #endif
        .onChange(of: session.casting.session?.id) { _, _ in seekPosition = session.casting.session?.position ?? 0 }
        .task(id: itemID) { if let client = session.client, let item = try? await client.item(id: itemID) { isAudio = item.isAudio } }
    }

    private func scan() {
        guard !scanning, let client = session.client else { return }
        scanning = true
        Task {
            defer { scanning = false }
            do { devices = try await client.scanCastDevices(); scanned = true; message = nil }
            catch { message = AppSession.message(error) }
        }
    }
    private func start(_ device: CastDevice) {
        perform {
            guard let client = session.client, let store = session.progress else { throw ClientError.unavailable }
            let item = try await client.item(id: itemID)
            let position = session.player.currentItem?.id == itemID ? session.player.seconds : item.progress.watched ? 0 : item.progress.seconds
            try await session.casting.start(item: item, device: device, position: position, client: client, store: store)
            session.player.pause()
        }
    }
    private func perform(_ operation: @escaping @MainActor () async throws -> Void) {
        Task { do { try await operation(); message = nil } catch { message = AppSession.message(error) } }
    }
}

#if os(iOS)
struct AudioRoutePicker: UIViewRepresentable {
    let video: Bool
    func makeUIView(context: Context) -> AVRoutePickerView {
        let view = AVRoutePickerView()
        view.prioritizesVideoDevices = video
        return view
    }
    func updateUIView(_ view: AVRoutePickerView, context: Context) { view.prioritizesVideoDevices = video }
}
#endif
