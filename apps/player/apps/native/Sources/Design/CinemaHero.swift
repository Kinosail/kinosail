import SwiftUI

/// Artwork and information keep separate, readable space on every device.
struct CinemaHero<Actions: View>: View {
    let item: MediaItem
    var title: String?
    var subtitle: String?
    var showsPlot = true
    @ViewBuilder let actions: () -> Actions
    @Environment(\.dynamicTypeSize) private var dynamicType
    @Environment(AppSession.self) private var session
    @State private var progress: WatchProgressSummary?
    #if os(tvOS)
    @ScaledMetric(relativeTo: .largeTitle) private var titleSize = 56.0
    #else
    @ScaledMetric(relativeTo: .largeTitle) private var titleSize = 36.0
    #endif

    var body: some View {
        CinemaHeroLayout {
            if !item.backdrop.isEmpty {
                Artwork(path: item.backdrop, ratio: 16 / 9, isBackdrop: true)
                    #if os(tvOS)
                    .frame(maxWidth: 720)
                    #endif
                    .overlay(alignment: .bottom) { posterProgress }
                    .clipShape(.rect(cornerRadius: 12))
            } else if !item.poster.isEmpty {
                Artwork(path: item.poster, symbol: item.kind.symbol, ratio: item.isAudio ? 1 : 2 / 3)
                    .frame(maxWidth: 240)
                    .overlay(alignment: .bottom) { posterProgress }
                    .clipShape(.rect(cornerRadius: 12))
            }
        } information: {
            VStack(alignment: .leading, spacing: 12) {
                VStack(alignment: .leading, spacing: 6) {
                    Text(title ?? item.title)
                        .font(.system(size: titleSize, weight: .bold, design: .rounded))
                        .fixedSize(horizontal: false, vertical: true)
                        .accessibilityAddTraits(.isHeader)
                    let summary = subtitle ?? item.subtitle
                    if !summary.isEmpty { Text(summary).font(.subheadline).foregroundStyle(KinoTheme.muted) }
                }
                if showsPlot && !item.plot.isEmpty {
                    Text(item.plot).font(.body).lineLimit(dynamicType.isAccessibilitySize ? nil : 3)
                        .fixedSize(horizontal: false, vertical: true)
                }
                if item.progress.seconds > 0 && !item.progress.watched {
                    if hasVideoArtwork {
                        Text(progress?.remainingLabel ?? "Continue from \(item.progress.seconds.clock)")
                            .font(.caption).foregroundStyle(KinoTheme.muted).monospacedDigit()
                    } else { WatchPosition(item: item) }
                }
                ViewThatFits(in: .horizontal) {
                    HStack(spacing: actionSpacing) { actions().fixedSize(horizontal: false, vertical: true) }
                    VStack(alignment: .leading, spacing: 12) { actions().fixedSize(horizontal: false, vertical: true) }
                }
                .controlSize(.large)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .foregroundStyle(KinoTheme.text)
        #if os(tvOS)
        .focusSection()
        #endif
        .task(id: "\(session.profileKey ?? ""):\(item.id):\(session.contentRevision)") {
            progress = nil
            guard hasVideoArtwork, item.progress.seconds > 0, !item.progress.watched,
                  let client = session.client else { return }
            do {
                let next = try await client.watchProgress(itemID: item.id)
                try Task.checkCancellation()
                progress = next
            } catch { /* The saved position remains available when duration is unknown. */ }
        }
    }

    @ViewBuilder private var posterProgress: some View {
        if hasVideoArtwork, item.progress.seconds > 0, !item.progress.watched,
           let fraction = progress?.fraction {
            ProgressView(value: fraction).tint(KinoTheme.signal)
                .progressViewStyle(.linear)
                .background(.black.opacity(0.65), in: Capsule())
                .padding(.horizontal, 12).padding(.bottom, 8)
                .accessibilityLabel("Watch progress")
                .accessibilityValue(progress?.remainingLabel ?? "")
        }
    }

    private var hasVideoArtwork: Bool {
        item.kind == .video && (!item.backdrop.isEmpty || !item.poster.isEmpty)
    }

    private var actionSpacing: CGFloat {
        #if os(tvOS)
        40
        #else
        12
        #endif
    }
}

struct CinemaHeroLayout<Art: View, Information: View>: View {
    @ViewBuilder let art: () -> Art
    @ViewBuilder let information: () -> Information
    @Environment(\.dynamicTypeSize) private var dynamicType
    @Environment(\.horizontalSizeClass) private var sizeClass

    var body: some View {
        let layout = wide ? AnyLayout(HStackLayout(alignment: .center, spacing: 32))
                          : AnyLayout(VStackLayout(alignment: .leading, spacing: 12))
        layout { art(); information() }.frame(maxWidth: .infinity, alignment: .leading)
    }
    private var wide: Bool {
        #if os(tvOS)
        !dynamicType.isAccessibilitySize
        #else
        sizeClass == .regular && !dynamicType.isAccessibilitySize
        #endif
    }
}
