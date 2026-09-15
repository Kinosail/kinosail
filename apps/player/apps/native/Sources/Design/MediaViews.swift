import SwiftUI

struct Artwork: View {
    let path: String
    var symbol = "film"
    var ratio: CGFloat = 2 / 3
    var dimension = 1600
    var fillsFrame = false
    var isBackdrop = false
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
                if let image { Image(uiImage: image).resizable().scaledToFill().id(imageIdentity).transition(.opacity) }
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
        if fillsFrame { Color.clear }
        else { Color.clear.aspectRatio(ratio, contentMode: .fit) }
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
    var body: some View {
        NavigationLink(value: resumesPlayback ? item.playingDestination : item.destination) {
            VStack(alignment: .leading, spacing: 10) {
                Artwork(path: landscape && !item.backdrop.isEmpty ? item.backdrop : item.poster,
                        symbol: item.kind.symbol, ratio: landscape ? 16 / 9 : item.isAudio ? 1 : 2 / 3,
                        dimension: landscape || dynamicTypeSize.isAccessibilitySize ? 1600 : 800)
                    .clipShape(.rect(cornerRadius: 12))
                VStack(alignment: .leading, spacing: 8) {
                    Text(item.title).font(.headline).foregroundStyle(KinoTheme.text)
                        .mediaLineLimit(2, accessibility: dynamicTypeSize.isAccessibilitySize)
                        .frame(maxWidth: .infinity, alignment: .leading)
                    if !item.subtitle.isEmpty {
                        Text(item.subtitle).font(.caption).foregroundStyle(KinoTheme.muted)
                            .mediaLineLimit(1, accessibility: dynamicTypeSize.isAccessibilitySize)
                    }
                    if resumesPlayback { WatchPosition(item: item) }
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
        #endif
        .accessibilityElement(children: .combine)
    }
}

struct MediaGrid: View {
    var landscape = false
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    let items: [MediaItem]
    var onFocus: ((MediaItem) -> Void)?
    var body: some View {
        LazyVGrid(columns: Self.columns(landscape: landscape, accessibility: dynamicTypeSize.isAccessibilitySize), alignment: .leading, spacing: 28) {
            ForEach(items) { MediaCard(item: $0, landscape: landscape, onFocus: onFocus) }
        }
        #if os(tvOS)
        .padding(.vertical, 24)
        .focusSection()
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

struct MediaShelf: View {
    @ScaledMetric(relativeTo: .headline) private var posterWidth = 164.0
    @ScaledMetric(relativeTo: .headline) private var landscapeWidth = 260.0
    let title: String
    let items: [MediaItem]
    var landscape = false
    var resumesPlayback = false
    var moreTitle: String?
    var moreDestination: ScreenDestination?
    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack(alignment: .firstTextBaseline) {
                Text(title).font(.title2.bold()).accessibilityAddTraits(.isHeader)
                Spacer()
                if let moreTitle, let moreDestination {
                    NavigationLink(moreTitle, value: moreDestination).font(.callout)
                        #if os(tvOS)
                        .buttonStyle(.bordered).tint(KinoTheme.raised).foregroundStyle(KinoTheme.text)
                        #endif
                }
            }
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: 18) {
                    ForEach(items) { item in MediaCard(item: item, landscape: landscape, resumesPlayback: resumesPlayback).frame(width: width) }
                }
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

struct RetryState: View {
    var title = "Couldn’t load this content"
    let message: String
    let retry: () -> Void
    var body: some View {
        VStack(spacing: 16) {
            Label(title, systemImage: "exclamationmark.circle").font(.headline)
            Text(message).foregroundStyle(KinoTheme.muted).multilineTextAlignment(.center)
            Button("Try again", action: retry).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.raised).foregroundStyle(KinoTheme.text)
        }
        .padding(32).frame(maxWidth: .infinity, minHeight: 220)
    }
}

enum LoadingLayout { case shelf, home, detail, grid }

struct LoadingState: View {
    var title = "Loading your library…"
    var layout = LoadingLayout.shelf
    @Environment(\.dynamicTypeSize) private var dynamicType
    @ScaledMetric(relativeTo: .headline) private var posterWidth = 164.0
    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            ProgressView(title).foregroundStyle(KinoTheme.muted)
            if layout == .home || layout == .detail {
                CinemaHeroLayout {
                    RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface).aspectRatio(16 / 9, contentMode: .fit)
                } information: {
                    featureInformation.frame(maxWidth: .infinity, alignment: .leading)
                }.accessibilityHidden(true)
            }
            if layout == .home {
                VStack(alignment: .leading, spacing: 16) {
                    line(width: 200, height: 28)
                    ForEach(0..<2) { _ in
                        HStack(spacing: 12) {
                            RoundedRectangle(cornerRadius: 8).fill(KinoTheme.surface)
                                .aspectRatio(16 / 9, contentMode: .fit).frame(width: 112)
                            VStack(alignment: .leading, spacing: 8) {
                                line(width: 180, height: 20)
                                line(width: 100, height: 14)
                                line(width: 240, height: 4)
                            }
                        }
                    }
                }.padding(.top, 20).accessibilityHidden(true)
            }
            if layout != .detail {
                line(width: 180, height: 24).accessibilityHidden(true)
                Group {
                    if layout == .grid {
                        LazyVGrid(columns: MediaGrid.columns(landscape: false, accessibility: dynamicType.isAccessibilitySize), alignment: .leading, spacing: 28) {
                            ForEach(0..<8) { _ in card }
                        }
                    } else {
                        ScrollView(.horizontal) {
                            HStack(alignment: .top, spacing: 18) {
                                ForEach(0..<4) { _ in card.frame(width: shelfWidth) }
                            }.padding(.vertical, 12)
                        }.scrollIndicators(.hidden).scrollDisabled(true)
                    }
                }.accessibilityHidden(true)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
    private var card: some View {
        VStack(alignment: .leading, spacing: 12) {
            RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface).aspectRatio(2 / 3, contentMode: .fit)
            line(width: 100, height: 16)
        }
    }
    private var featureInformation: some View {
        VStack(alignment: .leading, spacing: 16) {
            line(width: 260, height: 42)
            line(width: 160, height: 18)
            line(width: 100, height: 14)
            Capsule().fill(KinoTheme.raised).frame(height: 50)
        }.frame(maxWidth: .infinity, alignment: .leading)
    }
    private var shelfWidth: CGFloat {
        #if os(tvOS)
        230
        #else
        min(posterWidth, 260)
        #endif
    }
    private func line(width: CGFloat, height: CGFloat) -> some View {
        RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(maxWidth: width).frame(height: height)
    }
}

struct ResourceView<Value: Sendable, Content: View>: View {
    let identity: String
    var loadingLayout = LoadingLayout.shelf
    var allowsPullToRefresh = true
    let load: (CatalogPolicy) async throws -> Value
    @ViewBuilder let content: (Value) -> Content
    @State private var value: Value?
    @State private var failure: String?
    @State private var revision = 0
    @State private var generation = UUID()
    @State private var loadedIdentity: String?

    var body: some View {
        Group {
            if allowsPullToRefresh {
                resourceContent.refreshable { await refresh(force: true) }
            } else {
                resourceContent
            }
        }
        .task(id: "\(identity):\(revision)") { await refresh(force: revision > 0) }
    }

    private var resourceContent: some View {
        Group {
            if let value {
                VStack(alignment: .leading, spacing: 16) {
                    if let failure {
                        Text("Couldn’t refresh. \(failure)").font(.callout).foregroundStyle(KinoTheme.muted)
                        Button("Try again") { revision += 1 }
                    }
                    content(value)
                }
            } else if let failure { RetryState(message: failure) { revision += 1 } }
            else { LoadingState(layout: loadingLayout) }
        }
    }

    private func refresh(force: Bool) async {
        if loadedIdentity != identity { value = nil; failure = nil; loadedIdentity = identity }
        let attempt = UUID()
        generation = attempt
        do {
            if value == nil, let saved = try? await load(.cached) {
                try Task.checkCancellation()
                guard generation == attempt else { return }
                value = saved
            }
            let next = try await load(force ? .reload : .automatic)
            try Task.checkCancellation()
            guard generation == attempt else { return }
            value = next
            failure = nil
        } catch is CancellationError {}
        catch {
            if generation == attempt {
                if (error as? ClientError)?.discardsCachedContent == true { value = nil }
                failure = AppSession.message(error)
            }
        }
    }
}

extension MediaItem {
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
