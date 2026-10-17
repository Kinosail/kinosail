#if os(tvOS)
import SwiftUI

struct TVPlaybackOptionRows: View {
    let item: MediaItem
    @Environment(AppSession.self) private var session

    var body: some View {
        Section {
            if !session.player.subtitleTracks.isEmpty {
                NavigationLink { subtitles } label: { TVPlaybackValueLabel(title: "Subtitles", value: subtitleValue) }
            }
            if !session.player.audioTracks.isEmpty {
                NavigationLink { audio } label: { TVPlaybackValueLabel(title: "Audio", value: audioValue) }
            }
            NavigationLink { speed } label: { TVPlaybackValueLabel(title: "Playback speed", value: speedValue(session.player.playbackRate)) }
            if let chapters = session.player.source?.chapters, !chapters.isEmpty {
                NavigationLink { chapterList(chapters) } label: {
                    TVPlaybackValueLabel(title: "Chapters", value: chapters.count == 1 ? "1 chapter" : "\(chapters.count) chapters")
                }
            }
            NavigationLink("Bookmarks", value: ScreenDestination.bookmarks(item.id))
            NavigationLink("This title’s preferences") { PlaybackPreferencesScreen(itemID: item.id) }
        } header: {
            VStack(alignment: .leading, spacing: 4) {
                Text(session.player.usingCompatibility ? "Playing a compatible version" : "Playing the original file")
                if let progress = session.player.progressMessage { Text(progress) }
            }
            .font(.callout)
            .foregroundStyle(KinoTheme.muted)
            .textCase(nil)
            .padding(.bottom, 16)
        }
    }

    private var subtitleValue: String {
        session.player.subtitleTracks.first { $0.id == session.player.selectedSubtitleTrackID }?.title ?? "Off"
    }

    private var audioValue: String {
        session.player.audioTracks.first { $0.id == session.player.selectedAudioTrackID }?.title ?? session.player.audioTracks[0].title
    }

    private func speedValue(_ rate: Double) -> String { rate == 1 ? "Normal" : "\(rate.formatted())×" }

    private var subtitles: some View {
        let selected = session.player.selectedSubtitleTrackID
        let choices = [TVPlaybackChoice(id: "", title: "Off", selected: selected == nil)]
            + session.player.subtitleTracks.map { TVPlaybackChoice(id: $0.id, title: $0.title, selected: $0.id == selected) }
        let footer = session.player.externalCaptions
            ? "External subtitles are unavailable in Picture in Picture. Choose an embedded track to keep subtitles there." : nil
        return TVPlaybackChoiceScreen(title: "Subtitles", symbol: "captions.bubble", choices: choices, footer: footer) { id in
            try await session.player.selectSubtitleTrack(id: id.isEmpty ? nil : id)
        }
    }

    private var audio: some View {
        let selected = session.player.selectedAudioTrackID ?? session.player.audioTracks.first?.id
        let choices = session.player.audioTracks.map { TVPlaybackChoice(id: $0.id, title: $0.title, selected: $0.id == selected) }
        return TVPlaybackChoiceScreen(title: "Audio", symbol: "speaker.wave.2", choices: choices) { id in
            try session.player.selectAudioTrack(id: id)
        }
    }

    private var speed: some View {
        let current = session.player.playbackRate
        let choices = PlaybackSpeedScreen.rates.map { TVPlaybackChoice(id: String($0), title: speedValue($0), selected: $0 == current) }
        return TVPlaybackChoiceScreen(title: "Playback speed", symbol: "speedometer", choices: choices) { id in
            guard let rate = Double(id) else { throw ClientError.invalidInput("Choose a playback speed.") }
            try await session.player.changeRate(rate)
        }
    }

    private func chapterList(_ chapters: [Chapter]) -> some View {
        let choices = chapters.map { TVPlaybackChoice(id: String($0.start), title: "\($0.title) · \($0.start.clock)", selected: false) }
        return TVPlaybackChoiceScreen(title: "Chapters", symbol: "list.bullet", choices: choices) { id in
            guard let start = Double(id) else { throw ClientError.invalidInput("Choose a chapter.") }
            try await session.player.seek(to: start)
        }
    }
}

struct TVPlaybackValueLabel: View {
    let title: String
    let value: String

    var body: some View {
        HStack(spacing: 24) {
            Text(title).foregroundStyle(.primary)
            Spacer(minLength: 16)
            Text(value).foregroundStyle(.secondary).lineLimit(1).truncationMode(.tail)
        }
        .accessibilityElement(children: .combine)
    }
}

struct TVPlaybackChoice: Identifiable {
    let id: String
    let title: String
    let selected: Bool
}

struct TVPlaybackChoiceScreen: View {
    let title: String
    let symbol: String
    let choices: [TVPlaybackChoice]
    var footer: String?
    let choose: @MainActor (String) async throws -> Void
    @Environment(\.dismiss) private var dismiss
    @FocusState private var focused: String?
    @State private var changing = false
    @State private var message: String?

    var body: some View {
        List {
            if let message { Text(message).foregroundStyle(.secondary) }
            Section {
                ForEach(choices) { choice in
                    Button { pick(choice.id) } label: {
                        HStack {
                            Text(choice.title).foregroundStyle(.primary)
                            Spacer()
                            if choice.selected { Image(systemName: "checkmark").accessibilityLabel("Selected") }
                        }
                        .frame(minHeight: 44)
                    }
                    .focused($focused, equals: choice.id)
                    .accessibilityAddTraits(choice.selected ? .isSelected : [])
                    .disabled(changing)
                }
            } footer: {
                if let footer { Text(footer) }
            }
        }
        .defaultFocus($focused, choices.first(where: \.selected)?.id ?? choices.first?.id)
        .task { if let selected = choices.first(where: \.selected)?.id { focused = selected } }
        .tvOSConfigurationLayout(title: title, symbol: symbol)
        .configurationNavigationTitle(title)
    }

    private func pick(_ id: String) {
        changing = true
        Task {
            defer { changing = false }
            do { try await choose(id); message = nil; dismiss() }
            catch is CancellationError {} catch { message = AppSession.message(error) }
        }
    }
}
#endif
