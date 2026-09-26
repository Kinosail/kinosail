import SwiftUI

enum LoadingLayout { case shelf, home, homeAudio, detail, grid, squareGrid, musicGrid, album, show, list, playback, actor, collectionGrid }

struct LoadingState: View {
    var title = "Loading your library…"
    var layout = LoadingLayout.shelf
    @Environment(\.dynamicTypeSize) private var dynamicType
    @ScaledMetric(relativeTo: .headline) private var posterWidth = 164.0
    @ScaledMetric(relativeTo: .headline) private var landscapeWidth = 260.0
    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            if layout == .home || layout == .homeAudio {
                line(width: 180, height: 28).accessibilityHidden(true)
            }
            if layout == .home || layout == .homeAudio || layout == .detail {
                CinemaHeroLayout {
                    RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface)
                        .aspectRatio(layout == .homeAudio ? 1 : 16 / 9, contentMode: .fit)
                        .frame(maxWidth: layout == .homeAudio ? 240 : .infinity)
                } information: {
                    featureInformation.frame(maxWidth: .infinity, alignment: .leading)
                }.accessibilityHidden(true)
            }
            if layout == .home {
                VStack(alignment: .leading, spacing: 12) {
                    line(width: 200, height: 28)
                    ScrollView(.horizontal) {
                        HStack(alignment: .top, spacing: 18) {
                            ForEach(0..<4) { _ in card(ratio: 16 / 9, showsProgress: true).frame(width: continuationWidth) }
                        }
                        #if os(tvOS)
                        .padding(.horizontal, 24)
                        #endif
                        .padding(.vertical, 24)
                    }.scrollIndicators(.hidden).scrollDisabled(true)
                }.padding(.top, 12).accessibilityHidden(true)
            }
            if layout == .homeAudio {
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
                VStack(alignment: .leading, spacing: 16) {
                    line(width: 260, height: 42)
                    line(width: 360, height: 24)
                    Capsule().fill(KinoTheme.raised).frame(width: 280, height: 50)
                }
                .accessibilityHidden(true)
                HStack(alignment: .top, spacing: 32) {
                    VStack(alignment: .leading, spacing: 16) {
                        line(width: 140, height: 28)
                        ForEach(0..<3) { _ in
                            RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface).frame(width: 220, height: 64)
                        }
                    }
                    .frame(width: 220)
                    VStack(alignment: .leading, spacing: 16) {
                        line(width: 160, height: 28)
                        ScrollView(.horizontal) {
                            HStack(alignment: .top, spacing: 18) {
                                ForEach(0..<3) { _ in card(ratio: 16 / 9).frame(width: 390) }
                            }
                            .padding(.horizontal, 24)
                            .padding(.vertical, 24)
                        }
                        .scrollIndicators(.hidden).scrollDisabled(true)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
                .accessibilityHidden(true)
                #else
                CinemaHeroLayout {
                    RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface).aspectRatio(16 / 9, contentMode: .fit)
                } information: {
                    featureInformation.frame(maxWidth: .infinity, alignment: .leading)
                }.accessibilityHidden(true)
                line(width: 240, height: 48).accessibilityHidden(true)
                #endif
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
            if layout == .grid || layout == .squareGrid || layout == .musicGrid || showsGrid || layout == .actor || layout == .collectionGrid {
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
                    ForEach(0..<8) { _ in card(ratio: gridRatio, showsProgress: layout == .grid, showsSubtitle: layout != .grid && layout != .actor) }
                }
                #if os(tvOS)
                .padding(.vertical, 24)
                #endif
                .accessibilityHidden(true)
            }
            if layout == .shelf || layout == .home || layout == .homeAudio {
                ForEach(0..<(layout == .shelf ? 1 : 2), id: \.self) { _ in
                    line(width: 180, height: 28).accessibilityHidden(true)
                    ScrollView(.horizontal) {
                        HStack(alignment: .top, spacing: 18) {
                            ForEach(0..<4) { _ in card(ratio: layout == .homeAudio ? 1 : 2 / 3).frame(width: shelfWidth) }
                        }
                        #if os(tvOS)
                        .padding(.horizontal, 24)
                        #endif
                        .padding(.vertical, 24)
                    }
                    .scrollIndicators(.hidden).scrollDisabled(true).accessibilityHidden(true)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .skeletonLoading(layout == .playback ? "Opening media…" : title, shimmers: layout != .playback)
    }
    private func card(ratio: CGFloat = 2 / 3, showsProgress: Bool = false, showsSubtitle: Bool = true) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            RoundedRectangle(cornerRadius: 12).fill(KinoTheme.surface).aspectRatio(ratio, contentMode: .fit)
            VStack(alignment: .leading, spacing: 8) {
                line(width: 100, height: 20)
                if showsSubtitle { line(width: 80, height: 14) }
                if showsProgress { line(width: 140, height: 4) }
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
    private var continuationWidth: CGFloat {
        #if os(tvOS)
        390
        #else
        min(landscapeWidth, 300)
        #endif
    }
    private var gridRatio: CGFloat {
        if layout == .musicGrid || layout == .squareGrid { return 1 }
        return layout == .show ? 16 / 9 : 2 / 3
    }
    private var showsGrid: Bool {
        #if os(tvOS)
        false
        #else
        layout == .show
        #endif
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
