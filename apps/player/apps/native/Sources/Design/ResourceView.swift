import SwiftUI

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
