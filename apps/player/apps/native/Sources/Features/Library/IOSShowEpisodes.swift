#if os(iOS)
import SwiftUI

struct IOSShowEpisodes: View {
    let episodes: [MediaItem]
    let nextID: String
    let jumpTo: (Int) -> Void
    @Environment(AppSession.self) private var session
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @State private var downloadingSeason: Int?
    @State private var feedbackSeason: Int?
    @State private var feedback: String?

    private var groups: [ShowSeasonSelection.Group] { ShowSeasonSelection.groups(episodes) }

    var body: some View {
        VStack(alignment: .leading, spacing: 24) {
            VStack(alignment: .leading, spacing: 4) {
                Text("Episodes").font(.title2.bold()).accessibilityAddTraits(.isHeader)
                Text("\(episodes.count) available").font(.subheadline).foregroundStyle(KinoTheme.muted)
            }
            if groups.count > 1 {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Jump to season").font(.subheadline.weight(.semibold))
                    LazyVGrid(columns: [GridItem(.adaptive(minimum: dynamicTypeSize.isAccessibilitySize ? 240 : 132), spacing: 8)], spacing: 8) {
                        ForEach(groups) { group in
                            Button(group.title) { jumpTo(group.number) }
                                .buttonStyle(.bordered).buttonBorderShape(.capsule)
                                .tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
                                .frame(maxWidth: .infinity, minHeight: 44)
                        }
                    }
                }
            }
            if session.viewer?.downloads == true {
                Text("Season downloads use compatible video and all audio tracks. Open an episode to choose subtitles.")
                    .font(.caption).foregroundStyle(KinoTheme.muted)
            }
            LazyVStack(alignment: .leading, spacing: 28) {
                ForEach(groups) { group in
                    VStack(alignment: .leading, spacing: 12) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text(group.title).font(.title3.bold()).accessibilityAddTraits(.isHeader)
                            Text("\(group.episodes.count) \(group.episodes.count == 1 ? "episode" : "episodes")" +
                                 (group.watchedCount == 0 ? "" : " · \(group.watchedCount) watched"))
                                .font(.caption).foregroundStyle(KinoTheme.muted)
                        }
                        if session.viewer?.downloads == true {
                            Button { download(group) } label: {
                                if downloadingSeason == group.number { ProgressView() }
                                else { Label("Download season", systemImage: "arrow.down.circle") }
                            }
                            .buttonStyle(.bordered).frame(minHeight: 44)
                            .disabled(downloadingSeason != nil)
                            .accessibilityLabel("Download \(group.title)")
                        }
                        if feedbackSeason == group.number, let feedback {
                            Text(feedback).font(.callout).foregroundStyle(KinoTheme.muted)
                        }
                        Divider()
                        LazyVStack(alignment: .leading, spacing: 0) {
                            ForEach(group.episodes) { item in
                                IOSEpisodeRow(item: item, isNext: item.id == nextID)
                                Divider()
                            }
                        }
                    }
                    .id("season-\(group.number)")
                }
            }
        }
        .frame(maxWidth: 900, alignment: .leading)
        .foregroundStyle(KinoTheme.text)
    }

    private func download(_ group: ShowSeasonSelection.Group) {
        guard let client = session.client, downloadingSeason == nil else { return }
        downloadingSeason = group.number
        feedbackSeason = nil
        Task {
            defer { downloadingSeason = nil }
            do {
                try await session.downloads.enqueueEpisodes(group.episodes, quality: .compatible, client: client)
                feedback = "\(group.title) added to Downloads."
            } catch { feedback = AppSession.message(error) }
            feedbackSeason = group.number
        }
    }
}

private struct IOSEpisodeRow: View {
    let item: MediaItem
    let isNext: Bool
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @Environment(\.horizontalSizeClass) private var sizeClass

    private var title: String { ShowSeasonSelection.episodeTitle(item) }
    private var thumbnailWidth: CGFloat { sizeClass == .regular ? 112 : 76 }
    private var accessibilitySummary: String {
        let state = isNext ? "next" : item.progress.watched ? "watched" :
            item.progress.seconds > 0 ? "resume at \(item.progress.seconds.clock)" : ""
        return "Episode \(item.episode), \(title)" + (state.isEmpty ? "" : ", \(state)")
    }

    var body: some View {
        HStack(alignment: .center, spacing: 8) {
            NavigationLink(value: ScreenDestination.detail(item.id)) {
                HStack(alignment: .top, spacing: 12) {
                    if !dynamicTypeSize.isAccessibilitySize {
                        Color.clear.frame(width: thumbnailWidth, height: 0)
                    }
                    VStack(alignment: .leading, spacing: 4) {
                        Text(String(format: "%02d", item.episode))
                            .font(.caption.weight(.semibold)).monospacedDigit().foregroundStyle(KinoTheme.muted)
                        Text(title).font(.headline).foregroundStyle(KinoTheme.text)
                            .fixedSize(horizontal: false, vertical: true)
                        if !item.plot.isEmpty {
                            Text(item.plot).font(.caption).foregroundStyle(KinoTheme.muted)
                                .lineLimit(dynamicTypeSize.isAccessibilitySize ? nil : 3)
                        }
                        if isNext { Text("Next").font(.caption.weight(.semibold)).foregroundStyle(KinoTheme.signal) }
                        else if item.progress.watched { Text("Watched").font(.caption).foregroundStyle(KinoTheme.muted) }
                        else if item.progress.seconds > 0 {
                            Text("Resume · \(item.progress.seconds.clock)").font(.caption).foregroundStyle(KinoTheme.signal)
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
                .frame(maxWidth: .infinity, minHeight: 72, alignment: .leading)
                .background(alignment: .leading) {
                    if !dynamicTypeSize.isAccessibilitySize {
                        GeometryReader { geometry in
                            Artwork(path: item.landscapeArtwork, symbol: "play.rectangle", dimension: 500,
                                    fillsFrame: true, canvasSize: CGSize(width: thumbnailWidth, height: geometry.size.height))
                                .clipShape(.rect(cornerRadius: 8))
                        }
                        .frame(width: thumbnailWidth)
                    }
                }
                .contentShape(.rect)
            }
            .buttonStyle(.plain)
            .accessibilityLabel(accessibilitySummary)
            .accessibilityHint("Shows episode details")
            NavigationLink(value: item.playingDestination) {
                Image(systemName: "play.fill").font(.body.weight(.semibold))
                    .foregroundStyle(KinoTheme.signal)
                    .frame(width: 44, height: 44)
                    .background(KinoTheme.raised, in: Circle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel("\(item.playLabel) \(title)")
        }
        .padding(.vertical, 8)
    }
}

struct IOSShowCast: View {
    let people: [CastMember]
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    var body: some View {
        if !people.isEmpty {
            VStack(alignment: .leading, spacing: 12) {
                Text("Cast").font(.title2.bold()).accessibilityAddTraits(.isHeader)
                LazyVStack(alignment: .leading, spacing: 0) {
                    ForEach(Array(people.enumerated()), id: \.offset) { _, person in
                        NavigationLink(value: ScreenDestination.actor(person.name)) {
                            HStack(spacing: 12) {
                                if !dynamicTypeSize.isAccessibilitySize {
                                    Artwork(path: person.image, symbol: "person.fill", dimension: 400)
                                        .frame(width: 56).clipShape(.rect(cornerRadius: 8))
                                }
                                VStack(alignment: .leading, spacing: 4) {
                                    Text(person.name).font(.headline).foregroundStyle(KinoTheme.text)
                                    if !person.role.isEmpty {
                                        Text(person.role).font(.caption).foregroundStyle(KinoTheme.muted)
                                    }
                                }
                                Spacer(minLength: 0)
                                Image(systemName: "chevron.right").font(.caption.weight(.semibold))
                                    .foregroundStyle(KinoTheme.muted).accessibilityHidden(true)
                            }
                            .frame(maxWidth: .infinity, minHeight: 64, alignment: .leading)
                            .contentShape(.rect)
                        }
                        .buttonStyle(.plain)
                        .accessibilityElement(children: .combine)
                        .accessibilityHint("Shows titles in your library featuring this actor")
                        Divider()
                    }
                }
            }
            .frame(maxWidth: 900, alignment: .leading)
            .foregroundStyle(KinoTheme.text)
        }
    }
}
#endif
