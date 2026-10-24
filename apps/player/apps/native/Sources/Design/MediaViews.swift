import SwiftUI

struct MediaCard: View {
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @Environment(\.displayScale) private var displayScale
    #if os(tvOS)
    @FocusState private var focused: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    #endif
    let item: MediaItem
    var landscape = false
    var resumesPlayback = false
    var onFocus: ((MediaItem) -> Void)?
    var onQuickPlay: ((ScreenDestination) -> Void)?
    var opensShow = false
    #if os(tvOS)
    var playsOnSelect = false
    var takesFocus = false
    var onTookFocus: (() -> Void)?
    private var quickPlayAction: (() -> Void)? {
        guard let onQuickPlay, let destination = TVOSQuickPlay.destination(for: item) else { return nil }
        return { if focused { onQuickPlay(destination) } }
    }
    #endif
    private var destination: ScreenDestination {
        #if os(tvOS)
        if playsOnSelect { return item.playingDestination }
        #endif
        return resumesPlayback ? item.playingDestination : item.destination(inShows: opensShow)
    }
    var body: some View {
        NavigationLink(value: destination) {
            VStack(alignment: .leading, spacing: 10) {
                Artwork(path: landscape ? item.landscapeArtwork : item.cardPoster,
                        symbol: item.kind.symbol, ratio: landscape ? 16 / 9 : item.isAudio ? 1 : 2 / 3,
                        dimension: MediaArtworkSize.dimension(landscape: landscape, accessibility: dynamicTypeSize.isAccessibilitySize, displayScale: displayScale),
                        fillsFrame: item.kind == .video || item.kind == .show || (landscape && item.kind == .photo))
                    .background(KinoTheme.surface)
                    .overlay(alignment: .topTrailing) {
                        if item.isUnwatched && item.progress.seconds == 0 {
                            UnwatchedMark().accessibilityHidden(true)
                        }
                    }
                    .overlay(alignment: .bottom) {
                        if item.kind == .video && item.progress.seconds > 0 && !item.progress.watched {
                            WatchPosition(item: item, barOnly: true)
                                .padding(.horizontal, 12).padding(.bottom, 8)
                                .accessibilityHidden(true)
                        }
                    }
                    #if os(iOS)
                    .downloadBadge(item)
                    #endif
                    .clipShape(.rect(cornerRadius: 12))
                    .zoomSource(destination)
                    #if os(tvOS)
                    .overlay {
                        if focused {
                            RoundedRectangle(cornerRadius: 12)
                                .strokeBorder(KinoTheme.text, lineWidth: 4)
                                .allowsHitTesting(false)
                        }
                    }
                    .scaleEffect(focused ? 1.06 : 1)
                    .shadow(color: .black.opacity(focused ? 0.55 : 0), radius: focused ? 24 : 0, y: focused ? 16 : 0)
                    .animation(reduceMotion ? nil : .easeOut(duration: 0.18), value: focused)
                    #endif
                VStack(alignment: .leading, spacing: 8) {
                    Text(item.title).font(KinoTheme.cardTitle).foregroundStyle(KinoTheme.text)
                        .mediaLineLimit(2, accessibility: dynamicTypeSize.isAccessibilitySize)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .frame(minHeight: KinoTheme.mediaTitleHeight, alignment: .topLeading)
                    let subtitle = cardSubtitle
                    if !subtitle.isEmpty {
                        Text(subtitle).font(.caption).foregroundStyle(KinoTheme.muted)
                            .mediaLineLimit(1, accessibility: dynamicTypeSize.isAccessibilitySize)
                    }
                    #if os(tvOS)
                    if resumesPlayback && item.kind == .video && item.progress.seconds > 0 && !item.progress.watched {
                        WatchPosition(item: item, textOnly: true)
                    }
                    #endif
                    if item.kind != .video && item.kind != .show {
                        if resumesPlayback { WatchPosition(item: item) }
                        else if item.progress.seconds > 0 && !item.progress.watched {
                            Label("Resume · \(item.progress.seconds.clock)", systemImage: "play.fill")
                                .font(.caption.weight(.medium)).foregroundStyle(KinoTheme.signal).monospacedDigit()
                        } else if item.progress.watched {
                            Label("Watched", systemImage: "checkmark").font(.caption).foregroundStyle(KinoTheme.muted)
                        } else { Text(" ").font(.caption).accessibilityHidden(true) }
                    }
                }
            }
            .frame(maxWidth: .infinity, alignment: .topLeading)
            .contentShape(.rect)
        }
        #if os(tvOS)
        .buttonStyle(.borderless)
        .focused($focused)
        .onChange(of: focused) { _, value in if value { onFocus?(item) } }
        .task(id: takesFocus) {
            guard takesFocus else { return }
            focused = true
            onTookFocus?()
        }
        .onPlayPauseCommand(perform: quickPlayAction)
        .contextMenu {
            if playsOnSelect, let onQuickPlay {
                Button("Details", systemImage: "info.circle") { onQuickPlay(item.destination(inShows: opensShow)) }
            }
        }
        .accessibilityHint(playsOnSelect ? "Select to play. Press and hold for details."
                           : item.kind == .photo ? "Select to view photo."
                           : onQuickPlay != nil && TVOSQuickPlay.destination(for: item) != nil
                           ? "Select for details, or press Play/Pause to play." : "Select for details.")
        #else
        .buttonStyle(PressableCardStyle())
        #endif
        .accessibilityElement(children: .combine)
        .accessibilityValue(item.isUnwatched ? (item.progress.seconds > 0 ? "Unwatched, continue from \(item.progress.seconds.clock)" : "Unwatched")
                            : item.progress.watched && (item.kind == .video || item.kind == .show) ? "Watched" : "")
    }
}

extension MediaCard {
    var cardSubtitle: String {
        guard item.kind == .video || item.kind == .show else { return item.subtitle }
        guard !item.show.isEmpty, item.season > 0 || item.episode > 0 else { return item.subtitleWithoutYear }
        let code = "S\(item.season) E\(item.episode)"
        return resumesPlayback || opensShow ? "\(item.show) · \(code)" : code
    }
}

struct MediaGrid: View {
    var landscape = false
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    #if os(tvOS)
    @Namespace private var gridFocus
    #endif
    let items: [MediaItem]
    var onFocus: ((MediaItem) -> Void)?
    var onQuickPlay: ((ScreenDestination) -> Void)?
    var requestFirstCardFocus = false
    var opensShows = false
    #if os(tvOS)
    var takesFirstFocus = false
    var onTookFocus: (() -> Void)?
    #endif
    var body: some View {
        LazyVGrid(columns: Self.columns(landscape: landscape, accessibility: dynamicTypeSize.isAccessibilitySize), alignment: .leading, spacing: 28) {
            ForEach(Array(items.enumerated()), id: \.element.id) { index, item in
                #if os(tvOS)
                MediaCard(item: item, landscape: landscape, onFocus: onFocus, onQuickPlay: onQuickPlay,
                          opensShow: opensShows, takesFocus: takesFirstFocus && index == 0, onTookFocus: onTookFocus)
                    .prefetchArtwork(Array(items.dropFirst(index + 1).prefix(8)), landscape: landscape)
                    .prefersDefaultFocus(requestFirstCardFocus && item.id == items.first?.id, in: gridFocus)
                #else
                MediaCard(item: item, landscape: landscape, onFocus: onFocus, onQuickPlay: onQuickPlay,
                          opensShow: opensShows)
                    .prefetchArtwork(Array(items.dropFirst(index + 1).prefix(8)), landscape: landscape)
                #endif
            }
        }
        #if os(tvOS)
        .padding(.vertical, 24)
        .focusSection()
        .focusScope(gridFocus)
        #endif
        .prefetchArtwork(items, landscape: landscape)
    }
    static func columns(landscape: Bool, accessibility: Bool) -> [GridItem] {
        if accessibility { return [GridItem(.flexible(), alignment: .top)] }
        #if os(tvOS)
        let minimum: CGFloat = landscape ? 360 : 230
        let spacing: CGFloat = 32
        #else
        let minimum: CGFloat = landscape ? 280 : 96
        let spacing: CGFloat = landscape ? 20 : 12
        #endif
        return [GridItem(.adaptive(minimum: minimum), spacing: spacing, alignment: .top)]
    }
}

enum TVOSQuickPlay {
    static func destination(for item: MediaItem) -> ScreenDestination? {
        item.kind == .video || item.isAudio ? item.playingDestination : nil
    }
}

struct MediaShelf: View {
    @ScaledMetric(relativeTo: .headline) private var posterWidth = 164.0
    @ScaledMetric(relativeTo: .headline) private var landscapeWidth = 260.0
    let title: LocalizedStringKey
    let items: [MediaItem]
    var landscape = false
    var resumesPlayback = false
    var moreTitle: String?
    var moreDestination: ScreenDestination?
    var onQuickPlay: ((ScreenDestination) -> Void)?
    var opensShows = false
    var onFocus: ((MediaItem) -> Void)?
    #if os(tvOS)
    var playsOnSelect = false
    var takesFocus = false
    var onTookFocus: (() -> Void)?
    #endif
    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack(alignment: .firstTextBaseline) {
                Text(title).font(.title2.bold()).accessibilityAddTraits(.isHeader)
                Spacer()
                if let moreTitle, let moreDestination {
                    NavigationLink(moreTitle, value: moreDestination).font(.callout).frame(minHeight: 44)
                        #if os(tvOS)
                        .buttonStyle(.bordered).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
                        #endif
                }
            }
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: KinoTheme.shelfSpacing) {
                    ForEach(Array(items.enumerated()), id: \.element.id) { index, item in
                        #if os(tvOS)
                        MediaCard(item: item, landscape: landscape, resumesPlayback: resumesPlayback,
                                  onFocus: onFocus, onQuickPlay: onQuickPlay, opensShow: opensShows, playsOnSelect: playsOnSelect,
                                  takesFocus: takesFocus && index == 0, onTookFocus: onTookFocus).frame(width: width)
                            .prefetchArtwork(Array(items.dropFirst(index + 1).prefix(8)), landscape: landscape)
                        #else
                        MediaCard(item: item, landscape: landscape, resumesPlayback: resumesPlayback,
                                  onFocus: onFocus, onQuickPlay: onQuickPlay, opensShow: opensShows).frame(width: width)
                            .prefetchArtwork(Array(items.dropFirst(index + 1).prefix(8)), landscape: landscape)
                        #endif
                    }
                }
                #if os(tvOS)
                .padding(.horizontal, 24)
                #endif
                .padding(.vertical, 24)
                .scrollTargetLayout()
            }
            .scrollIndicators(.hidden)
            .scrollTargetBehavior(.viewAligned)
            #if os(tvOS)
            .scrollClipDisabled()
            .padding(.horizontal, -24)
            #endif
            #if os(iOS)
            .scrollBounceBehavior(.basedOnSize, axes: .vertical)
            .contentShape(.interaction, .rect)
            #endif
        }
        #if os(tvOS)
        .focusSection()
        #endif
        .prefetchArtwork(items, landscape: landscape)
    }
    private var width: CGFloat {
        #if os(tvOS)
        landscape ? 390 : 230
        #else
        landscape ? min(landscapeWidth, 300) : min(posterWidth, 260)
        #endif
    }
}

extension MediaItem {
    func destination(inShows: Bool) -> ScreenDestination {
        #if os(tvOS)
        if kind == .photo { return .photos(id) }
        #endif
        return inShows && !showID.isEmpty ? .show(showID) : destination
    }
    var destination: ScreenDestination {
        kind == .show ? .show(showID.isEmpty ? id : showID) : .detail(id)
    }
    var playingDestination: ScreenDestination {
        switch kind {
        case .photo: .photos(id)
        case .book: .reader(id)
        case .show: .show(showID.isEmpty ? id : showID)
        case .music, .audiobook: .audio(id)
        case .video: .playback(id)
        }
    }
}

extension Double {
    var clock: String {
        guard isFinite, self >= 0, self <= 31_536_000 else { return "0:00" }
        let seconds = Int(self)
        return seconds >= 3600 ? String(format: "%d:%02d:%02d", seconds / 3600, seconds / 60 % 60, seconds % 60)
            : String(format: "%d:%02d", seconds / 60, seconds % 60)
    }
}

extension View {
    @ViewBuilder func mediaLineLimit(_ lines: Int, accessibility: Bool) -> some View {
        if accessibility { lineLimit(nil) }
        else { lineLimit(lines, reservesSpace: true) }
    }
}
