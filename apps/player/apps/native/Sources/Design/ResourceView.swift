import SwiftUI

struct RetryState: View {
    var title = "Couldn’t load this content"
    let message: String
    let retry: () -> Void
    var body: some View {
        VStack(spacing: 16) {
            Label(title, systemImage: "exclamationmark.circle").font(.headline)
            Text(message).foregroundStyle(KinoTheme.muted).multilineTextAlignment(.center)
            Button("Try again", action: retry).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
        }
        .padding(32).frame(maxWidth: .infinity, minHeight: 220)
    }
}

enum LoadingLayout { case shelf, home, homeAudio, detail, grid, squareGrid, musicGrid, album, show, list, playback, actor, collectionGrid }

struct LoadingState: View {
    var title = "Loading your library…"
    var layout = LoadingLayout.shelf
    @Environment(\.dynamicTypeSize) private var dynamicType
    @ScaledMetric(relativeTo: .headline) private var posterWidth = 164.0
    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            if layout == .home || layout == .homeAudio || layout == .detail {
                CinemaHeroLayout {
                    RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface)
                        .aspectRatio(layout == .homeAudio ? 1 : 16 / 9, contentMode: .fit)
                        .frame(maxWidth: layout == .homeAudio ? 240 : .infinity)
                } information: {
                    featureInformation.frame(maxWidth: .infinity, alignment: .leading)
                }.accessibilityHidden(true)
            }
            if layout == .home || layout == .homeAudio {
                VStack(alignment: .leading, spacing: 12) {
                    line(width: 200, height: 28)
                    LazyVGrid(columns: ResumeRows.columns(accessibility: dynamicType.isAccessibilitySize), alignment: .leading, spacing: 16) {
                        ForEach(0..<2) { _ in resumeRow }
                    }
                }.padding(.top, 12).accessibilityHidden(true)
            }
            if layout == .detail {
                VStack(alignment: .leading, spacing: 20) {
                    line(width: 340, height: 20)
                    line(width: 260, height: 16)
                    line(width: 180, height: 16)
                }.padding(.top, 8).accessibilityHidden(true)
            }
            if layout == .show {
                #if os(tvOS)
                featureInformation.accessibilityHidden(true)
                #else
                CinemaHeroLayout {
                    RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface).aspectRatio(16 / 9, contentMode: .fit)
                } information: {
                    featureInformation.frame(maxWidth: .infinity, alignment: .leading)
                }.accessibilityHidden(true)
                #endif
                line(width: 240, height: 48).accessibilityHidden(true)
            }
            if layout == .album {
                #if os(tvOS)
                HStack(alignment: .bottom, spacing: 32) {
                    RoundedRectangle(cornerRadius: 16).fill(KinoTheme.surface)
                        .aspectRatio(1, contentMode: .fit).frame(width: 260)
                    VStack(alignment: .leading, spacing: 8) {
                        line(width: 300, height: 42)
                        line(width: 180, height: 24)
                        line(width: 90, height: 18)
                    }
                }.accessibilityHidden(true)
                line(width: 100, height: 28).accessibilityHidden(true)
                #else
                RoundedRectangle(cornerRadius: 16).fill(KinoTheme.surface)
                    .aspectRatio(1, contentMode: .fit).frame(maxWidth: 360).accessibilityHidden(true)
                line(width: 300, height: 42).accessibilityHidden(true)
                line(width: 180, height: 20).accessibilityHidden(true)
                #endif
                ForEach(0..<4) { _ in
                    HStack(spacing: 20) {
                        line(width: 30, height: 20)
                        VStack(alignment: .leading, spacing: 8) {
                            line(width: 180, height: 20)
                            line(width: 120, height: 14)
                        }
                        Spacer()
                        line(width: 20, height: 20)
                    }.frame(minHeight: 56).accessibilityHidden(true)
                    Divider().accessibilityHidden(true)
                }
            }
            if layout == .list {
                #if os(tvOS)
                line(width: 260, height: 42).accessibilityHidden(true)
                #endif
                ForEach(0..<6) { _ in
                    HStack(spacing: 12) {
                        line(width: 24, height: 24)
                        line(width: 220, height: 24)
                    }.frame(minHeight: 48)
                    #if os(tvOS)
                    .padding(12)
                    .frame(maxWidth: 1100, alignment: .leading)
                    #else
                    .frame(maxWidth: .infinity, alignment: .leading)
                    #endif
                    .accessibilityHidden(true)
                    #if os(iOS)
                    Divider().accessibilityHidden(true)
                    #endif
                }
            }
            if layout == .playback {
                Text("Opening media…").foregroundStyle(KinoTheme.muted).frame(maxWidth: .infinity, minHeight: 220)
            }
            if layout == .grid || layout == .squareGrid || layout == .musicGrid || layout == .show || layout == .actor || layout == .collectionGrid {
                if layout == .musicGrid {
                    #if os(tvOS)
                    HStack { line(width: 180, height: 42); Spacer(); Capsule().fill(KinoTheme.raised).frame(width: 200, height: 48) }.accessibilityHidden(true)
                    #else
                    line(width: 160, height: 44).accessibilityHidden(true)
                    #endif
                }
                if layout == .actor {
                    #if os(tvOS)
                    HStack(alignment: .bottom, spacing: 32) {
                        RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface).aspectRatio(2 / 3, contentMode: .fit).frame(width: 200)
                        VStack(alignment: .leading, spacing: 8) { line(width: 260, height: 42); line(width: 160, height: 20) }
                    }.accessibilityHidden(true)
                    #else
                    RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface).aspectRatio(2 / 3, contentMode: .fit).frame(width: 200).accessibilityHidden(true)
                    line(width: 180, height: 28).accessibilityHidden(true)
                    #endif
                    line(width: 120, height: 24).accessibilityHidden(true)
                }
                #if os(tvOS)
                if layout == .collectionGrid { line(width: 260, height: 42).accessibilityHidden(true) }
                #endif
                LazyVGrid(columns: MediaGrid.columns(landscape: layout == .show, accessibility: dynamicType.isAccessibilitySize), alignment: .leading, spacing: 28) {
                    ForEach(0..<8) { _ in card(ratio: gridRatio) }
                }
                #if os(tvOS)
                .padding(.vertical, 24)
                #endif
                .accessibilityHidden(true)
            }
            if layout == .shelf || layout == .home || layout == .homeAudio {
                line(width: 180, height: 28).accessibilityHidden(true)
                ScrollView(.horizontal) {
                    HStack(alignment: .top, spacing: 18) {
                        ForEach(0..<4) { _ in card(ratio: layout == .homeAudio ? 1 : 2 / 3).frame(width: shelfWidth) }
                    }
                    #if os(tvOS)
                    .padding(.horizontal, 24)
                    #endif
                    .padding(.vertical, 24)
                }.scrollIndicators(.hidden).scrollDisabled(true).accessibilityHidden(true)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .skeletonLoading(layout == .playback ? "Opening media…" : title, shimmers: layout != .playback)
    }
    private func card(ratio: CGFloat = 2 / 3) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface).aspectRatio(ratio, contentMode: .fit)
            VStack(alignment: .leading, spacing: 8) {
                line(width: 100, height: 20)
                line(width: 80, height: 14)
            }
            #if os(tvOS)
            .padding([.horizontal, .bottom], 12)
            #endif
        }
    }
    private var resumeRow: some View {
        HStack(spacing: 12) {
            if !dynamicType.isAccessibilitySize {
                RoundedRectangle(cornerRadius: 8).fill(KinoTheme.surface)
                    .aspectRatio(layout == .homeAudio ? 1 : 16 / 9, contentMode: .fit).frame(width: resumeArtworkWidth)
            }
            VStack(alignment: .leading, spacing: 4) {
                line(width: 180, height: 20)
                line(width: 100, height: 14)
                line(width: 240, height: 4)
            }.frame(maxWidth: .infinity, alignment: .leading)
            line(width: 16, height: 16)
        }
        .frame(maxWidth: .infinity, minHeight: 80, alignment: .leading)
        #if os(tvOS)
        .padding(12)
        #endif
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
    private var gridRatio: CGFloat {
        if layout == .musicGrid || layout == .squareGrid { return 1 }
        return layout == .show ? 16 / 9 : 2 / 3
    }
    private var resumeArtworkWidth: CGFloat {
        #if os(tvOS)
        196
        #else
        112
        #endif
    }
    private func line(width: CGFloat, height: CGFloat) -> some View {
        RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(maxWidth: width).frame(height: height)
    }
}

struct ResourceView<Value: Sendable, Content: View>: View {
    let identity: String
    var refreshID = ""
    var loadingLayout = LoadingLayout.shelf
    var allowsPullToRefresh = true
    var revalidates = true
    let load: (CatalogPolicy) async throws -> Value
    @ViewBuilder let content: (Value) -> Content
    @Environment(\.scenePhase) private var scenePhase
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
        .task(id: "\(identity):\(refreshID):\(revision):\(revalidates ? String(describing: scenePhase) : "once")") {
            guard !revalidates || scenePhase == .active else { return }
            await refresh(force: revision > 0)
            while revalidates && !Task.isCancelled {
                do { try await Task.sleep(for: .seconds(60)) }
                catch { return }
                await refresh(force: false)
            }
        }
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
