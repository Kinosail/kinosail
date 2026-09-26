import SwiftUI

struct Artwork: View {
    let path: String
    var symbol = "film"
    var ratio: CGFloat = 2 / 3
    var dimension = 1600
    var fillsFrame = false
    var isBackdrop = false
    var canvasSize: CGSize? = nil
    @Environment(AppSession.self) private var session
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var image: UIImage?
    @State private var imageIdentity = UUID()
    @State private var loadedProfileKey: String?
    @State private var loading = true
    @State private var generation = UUID()

    var body: some View {
        canvas
            .overlay {
                if let image {
                    Image(uiImage: image).resizable()
                        .aspectRatio(contentMode: Self.contentMode(fillsFrame: fillsFrame))
                        .id(imageIdentity).transition(.opacity)
                }
                else if !isBackdrop { Rectangle().fill(KinoTheme.surface).overlay { if !loading { Image(systemName: symbol).font(.largeTitle).foregroundStyle(KinoTheme.muted) } } }
            }
            .clipped()
            .accessibilityHidden(true)
            .task(id: "\(session.profileKey ?? ""):\(path):\(dimension)") {
                let attempt = UUID()
                generation = attempt
                if !isBackdrop || loadedProfileKey != session.profileKey || path.isEmpty { image = nil }
                loadedProfileKey = session.profileKey
                loading = !path.isEmpty
                defer { if generation == attempt { loading = false } }
                guard !path.isEmpty, let client = session.client else { return }
                do {
                    let decoded = try await session.artwork.image(path: path, client: client, dimension: dimension)
                    try Task.checkCancellation()
                    guard generation == attempt else { return }
                    withAnimation(isBackdrop && !reduceMotion ? .easeInOut(duration: 0.45) : nil) {
                        image = UIImage(cgImage: decoded)
                        imageIdentity = UUID()
                    }
                } catch {
                    if generation == attempt && !Task.isCancelled { image = nil }
                }
            }
    }
    @ViewBuilder private var canvas: some View {
        if let canvasSize {
            Color.clear.frame(width: canvasSize.width, height: canvasSize.height)
        } else {
            Color.clear.aspectRatio(ratio, contentMode: .fit)
        }
    }

    static func contentMode(fillsFrame: Bool) -> ContentMode {
        fillsFrame ? .fill : .fit
    }
}

struct MediaCard: View {
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    #if os(tvOS)
    @FocusState private var focused: Bool
    #endif
    let item: MediaItem
    var landscape = false
    var resumesPlayback = false
    var onFocus: ((MediaItem) -> Void)?
    var onQuickPlay: ((ScreenDestination) -> Void)?
    var opensShow = false
    #if os(tvOS)
    private var quickPlayAction: (() -> Void)? {
        guard let onQuickPlay, let destination = TVOSQuickPlay.destination(for: item) else { return nil }
        return { if focused { onQuickPlay(destination) } }
    }
    #endif
    var body: some View {
        NavigationLink(value: resumesPlayback ? item.playingDestination : item.destination(inShows: opensShow)) {
            VStack(alignment: .leading, spacing: 10) {
                let usesBackdrop = landscape && !item.backdrop.isEmpty
                Artwork(path: usesBackdrop ? item.backdrop : item.poster,
                        symbol: item.kind.symbol, ratio: landscape ? 16 / 9 : item.isAudio ? 1 : 2 / 3,
                        dimension: landscape || dynamicTypeSize.isAccessibilitySize ? 1600 : 800,
                        fillsFrame: landscape && item.kind == .photo,
                        isBackdrop: usesBackdrop)
                    .background(landscape && !usesBackdrop ? KinoTheme.surface : .clear)
                    .overlay(alignment: .topTrailing) {
                        if item.isUnwatched {
                            UnwatchedCorner().fill(KinoTheme.signal).frame(width: 32, height: 32)
                                .accessibilityHidden(true)
                        }
                    }
                    .clipShape(.rect(cornerRadius: 12))
                VStack(alignment: .leading, spacing: 8) {
                    Text(item.title).font(.headline).foregroundStyle(KinoTheme.text)
                        .mediaLineLimit(2, accessibility: dynamicTypeSize.isAccessibilitySize)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    if !item.subtitle.isEmpty {
                        Text(item.subtitle).font(.caption).foregroundStyle(KinoTheme.muted)
                            .mediaLineLimit(1, accessibility: dynamicTypeSize.isAccessibilitySize)
                    }
                    if item.kind == .video || item.kind == .show {
                        if item.kind == .video && item.progress.seconds > 0 && !item.progress.watched {
                            WatchPosition(item: item, barOnly: true)
                        } else { Color.clear.frame(height: 6).accessibilityHidden(true) }
                    } else if resumesPlayback { WatchPosition(item: item) }
                    else if item.progress.seconds > 0 && !item.progress.watched {
                        Label("Resume · \(item.progress.seconds.clock)", systemImage: "play.fill")
                            .font(.caption.weight(.medium)).foregroundStyle(KinoTheme.signal).monospacedDigit()
                    } else if item.progress.watched {
                        Label("Watched", systemImage: "checkmark").font(.caption).foregroundStyle(KinoTheme.muted)
                    } else { Text(" ").font(.caption).accessibilityHidden(true) }
                }
                #if os(tvOS)
                .padding([.horizontal, .bottom], 12)
                #endif
            }
            .frame(maxWidth: .infinity, alignment: .topLeading)
            .contentShape(.rect)
        }
        #if os(iOS)
        .buttonStyle(.plain)
        #else
        .buttonStyle(.card)
        .focused($focused)
        .onChange(of: focused) { _, value in if value { onFocus?(item) } }
        .onPlayPauseCommand(perform: quickPlayAction)
        .accessibilityHint(item.kind == .photo ? "Select to view photo."
                           : onQuickPlay != nil && TVOSQuickPlay.destination(for: item) != nil
                           ? "Select for details, or press Play/Pause to play." : "Select for details.")
        #endif
        .accessibilityElement(children: .combine)
        .accessibilityValue(item.isUnwatched ? (item.progress.seconds > 0 ? "Unwatched, continue from \(item.progress.seconds.clock)" : "Unwatched")
                            : item.progress.watched && (item.kind == .video || item.kind == .show) ? "Watched" : "")
    }
}

private struct UnwatchedCorner: Shape {
    func path(in rect: CGRect) -> Path {
        Path { path in
            path.move(to: CGPoint(x: rect.minX, y: rect.minY))
            path.addLine(to: CGPoint(x: rect.maxX, y: rect.minY))
            path.addLine(to: CGPoint(x: rect.maxX, y: rect.maxY))
            path.closeSubpath()
        }
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
    var body: some View {
        LazyVGrid(columns: Self.columns(landscape: landscape, accessibility: dynamicTypeSize.isAccessibilitySize), alignment: .leading, spacing: 28) {
            ForEach(items) { item in
                MediaCard(item: item, landscape: landscape, onFocus: onFocus, onQuickPlay: onQuickPlay,
                          opensShow: opensShows)
                    #if os(tvOS)
                    .prefersDefaultFocus(requestFirstCardFocus && item.id == items.first?.id, in: gridFocus)
                    #endif
            }
        }
        #if os(tvOS)
        .padding(.vertical, 24)
        .focusSection()
        .focusScope(gridFocus)
        #endif
    }
    static func columns(landscape: Bool, accessibility: Bool) -> [GridItem] {
        if accessibility { return [GridItem(.flexible(), alignment: .top)] }
        #if os(tvOS)
        let minimum: CGFloat = landscape ? 360 : 230
        #else
        let minimum: CGFloat = landscape ? 280 : 144
        #endif
        return [GridItem(.adaptive(minimum: minimum), spacing: 20, alignment: .top)]
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
    let title: String
    let items: [MediaItem]
    var landscape = false
    var resumesPlayback = false
    var moreTitle: String?
    var moreDestination: ScreenDestination?
    var onQuickPlay: ((ScreenDestination) -> Void)?
    var opensShows = false
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
                LazyHStack(alignment: .top, spacing: 18) {
                    ForEach(items) { item in MediaCard(item: item, landscape: landscape, resumesPlayback: resumesPlayback,
                                                       onQuickPlay: onQuickPlay, opensShow: opensShows).frame(width: width) }
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
            #endif
            #if os(iOS)
            .scrollBounceBehavior(.basedOnSize, axes: .vertical)
            .contentShape(.interaction, .rect)
            #endif
        }
        #if os(tvOS)
        .focusSection()
        #endif
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

private extension View {
    @ViewBuilder func mediaLineLimit(_ lines: Int, accessibility: Bool) -> some View {
        if accessibility { lineLimit(nil) }
        else { lineLimit(lines, reservesSpace: true) }
    }
}
