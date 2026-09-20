import SwiftUI

struct PlaybackTracksScreen: View {
    @Environment(\.dismiss) private var dismiss
    var body: some View {
        List { PlaybackTrackSections() }
            .tvOSConfigurationLayout(title: "Audio & subtitles", symbol: "captions.bubble")
            .navigationTitle("Audio & subtitles")
            .toolbar { ToolbarItem(placement: .confirmationAction) { Button("Done") { dismiss() } } }
    }
}

struct PlaybackTrackSections: View {
    @Environment(AppSession.self) private var session
    @State private var changing = false
    @State private var message: String?

    var body: some View {
        Group {
            if let message { Section { Text(message).foregroundStyle(.secondary) } }
            if !session.player.audioTracks.isEmpty {
                Section("Audio") {
                    ForEach(session.player.audioTracks) { track in
                        choice(track.title, selected: session.player.selectedAudioTrackID == track.id) {
                            try session.player.selectAudioTrack(id: track.id)
                        }
                    }
                }
            }
            if !session.player.subtitleTracks.isEmpty {
                Section {
                    choice("Off", selected: session.player.selectedSubtitleTrackID == nil) {
                        try await session.player.selectSubtitleTrack(id: nil)
                    }
                    ForEach(session.player.subtitleTracks) { track in
                        choice(track.title, selected: session.player.selectedSubtitleTrackID == track.id) {
                            try await session.player.selectSubtitleTrack(id: track.id)
                        }
                    }
                } header: { Text("Subtitles") } footer: {
                    if session.player.externalCaptions {
                        Text("External captions appear here. Choose an embedded subtitle track for captions in Picture in Picture.")
                    }
                }
            }
            if changing { Section { ProgressView("Changing track…") } }
        }
    }

    private func choice(_ title: String, selected: Bool, action: @escaping @MainActor () async throws -> Void) -> some View {
        Button {
            changing = true
            Task {
                defer { changing = false }
                do { try await action(); message = nil }
                catch is CancellationError {} catch { message = AppSession.message(error) }
            }
        } label: {
            HStack {
                Text(title).foregroundStyle(.primary)
                Spacer()
                if selected { Image(systemName: "checkmark").accessibilityHidden(true) }
            }
            .frame(minHeight: 44)
        }
        .accessibilityAddTraits(selected ? .isSelected : [])
        .disabled(changing)
    }
}

struct PlaybackSpeedScreen: View {
    static let rates = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3]
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var changing = false
    @State private var message: String?

    var body: some View {
        List {
            if let message { Text(message).foregroundStyle(.secondary) }
            ForEach(Self.rates, id: \.self) { rate in
                Button {
                    changing = true
                    Task {
                        defer { changing = false }
                        do { try await session.player.changeRate(rate); dismiss() }
                        catch is CancellationError {} catch { message = AppSession.message(error) }
                    }
                } label: {
                    HStack {
                        Text(rate == 1 ? "Normal" : "\(rate.formatted())×").foregroundStyle(.primary)
                        Spacer()
                        if session.player.playbackRate == rate { Image(systemName: "checkmark").accessibilityHidden(true) }
                    }
                    .frame(minHeight: 44)
                }
                .accessibilityAddTraits(session.player.playbackRate == rate ? .isSelected : [])
                .disabled(changing)
            }
            if changing { ProgressView("Changing speed…") }
        }
        .tvOSConfigurationLayout(title: "Playback speed", symbol: "speedometer")
        .navigationTitle("Playback speed")
        .toolbar { ToolbarItem(placement: .confirmationAction) { Button("Done") { dismiss() } } }
    }
}
