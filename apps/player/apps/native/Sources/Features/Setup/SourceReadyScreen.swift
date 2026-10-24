import SwiftUI

struct SourceReadyScreen: View {
    let account: SourceAccount
    let earlier: [SourceAccount]
    let addAnother: () async throws -> Void
    let start: () async throws -> Void
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var preview: SourcePreview?
    @State private var posters: [UIImage] = []
    @State private var loading = true
    @State private var working = false
    @State private var failure: String?

    var body: some View {
        layout
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            .sensoryFeedback(.success, trigger: loading) { old, new in old && !new }
            #endif
            .cinemaBackground()
            .navigationBarBackButtonHidden()
            .task { await load() }
    }

    @ViewBuilder private var layout: some View {
        #if os(tvOS)
        HStack(spacing: 96) {
            PosterFan(posters: posters, loading: loading).frame(width: 820, height: 520)
            VStack(alignment: .leading, spacing: 36) { message; actions }.frame(width: 640, alignment: .leading)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        #else
        ScrollView {
            VStack(spacing: 28) {
                PosterFan(posters: posters, loading: loading).frame(height: UIDevice.current.userInterfaceIdiom == .pad ? 380 : 250).padding(.top, 12)
                message
                actions
            }
            .padding(.horizontal, KinoTheme.contentPadding)
            .padding(.vertical, 24)
            .frame(maxWidth: 560)
            .frame(maxWidth: .infinity)
        }
        .defaultScrollAnchor(.center, for: .alignment)
        #endif
    }

    private var message: some View {
        VStack(alignment: tv ? .leading : .center, spacing: 10) {
            Label("Connected", systemImage: "checkmark.seal.fill")
                .font(.subheadline.weight(.semibold)).foregroundStyle(KinoTheme.signal)
            Text("You’re all set").font(.system(.largeTitle, design: .rounded).bold()).foregroundStyle(KinoTheme.text)
                .accessibilityAddTraits(.isHeader)
            Text(summary).font(.body).foregroundStyle(KinoTheme.muted).multilineTextAlignment(tv ? .leading : .center)
                .contentTransition(.opacity)
        }
    }

    private var actions: some View {
        VStack(spacing: 12) {
            Button { run(start) } label: {
                Label("Start watching", systemImage: "play.fill").font(.headline).foregroundStyle(KinoTheme.actionInk)
                    .opacity(working ? 0 : 1)
                    .overlay { if working { ProgressView().tint(KinoTheme.signalInk) } }
                    .frame(maxWidth: .infinity, minHeight: 44)
            }
            .primaryCapsule()
            Button { run(addAnother) } label: {
                Label("Add another server", systemImage: "plus").font(.headline).frame(maxWidth: .infinity, minHeight: 44)
            }
            .buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint)
            .foregroundStyle(KinoTheme.text)
            if !earlier.isEmpty {
                Text("Also in your library: \(earlier.map(\.name).joined(separator: ", "))")
                    .font(.footnote).foregroundStyle(KinoTheme.muted).multilineTextAlignment(.center)
            }
            if let failure { Text(failure).font(.callout).foregroundStyle(.red) }
        }
        .disabled(working)
    }

    private var tv: Bool {
        #if os(tvOS)
        true
        #else
        false
        #endif
    }

    private var summary: String {
        let source = account.name == account.kind.title ? "\(account.kind.title) · signed in as \(account.userName)"
            : "\(account.name) · \(account.kind.title) · \(account.userName)"
        guard let preview else { return loading ? "\(source)\nGetting your library ready…" : source }
        var parts: [String] = []
        if preview.movies > 0 { parts.append(preview.movies == 1 ? "1 movie" : "\(preview.movies) movies") }
        if preview.shows > 0 { parts.append(preview.shows == 1 ? "1 show" : "\(preview.shows) shows") }
        return parts.isEmpty ? "\(source)\nThis library has no movies or shows yet." : "\(source)\n\(parts.joined(separator: " · ")) ready to watch"
    }

    private func load() async {
        defer { withAnimation(.easeOut(duration: 0.3)) { loading = false } }
        Task { await DeviceLibraryHost.shared.warm(account) }
        guard let value = try? await SourceConnect.preview(account) else { return }
        let images = value.posters.compactMap(UIImage.init(data:))
        withAnimation(reduceMotion ? .easeInOut(duration: 0.3) : .spring(duration: 0.8, bounce: 0.25)) {
            preview = value
            posters = images
        }
    }

    private func run(_ work: @escaping () async throws -> Void) {
        guard !working else { return }
        working = true
        failure = nil
        Task {
            defer { working = false }
            do { try await work() } catch { failure = AppSession.message(error) }
        }
    }
}

private struct PosterFan: View {
    let posters: [UIImage]
    let loading: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        GeometryReader { proxy in
            let height = proxy.size.height * 0.86
            let width = height * 2 / 3
            let count = max(posters.count, loading ? 3 : 0)
            let spread = count > 1 ? min(width * 0.52, max(0, proxy.size.width - width * 1.6 - 16) / Double(count - 1)) : 0
            ZStack {
                ForEach(0..<count, id: \.self) { index in
                    let offset = Double(index) - Double(count - 1) / 2
                    poster(index).frame(width: width, height: height)
                        .clipShape(.rect(cornerRadius: 14, style: .continuous))
                        .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).strokeBorder(.white.opacity(0.12), lineWidth: 1))
                        .shadow(color: .black.opacity(0.45), radius: 18, y: 10)
                        .rotationEffect(.degrees(reduceMotion ? 0 : offset * 7), anchor: .bottom)
                        .offset(x: offset * spread, y: abs(offset) * 10)
                        .zIndex(-abs(offset))
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
        .accessibilityHidden(true)
    }

    @ViewBuilder private func poster(_ index: Int) -> some View {
        if index < posters.count {
            Image(uiImage: posters[index]).resizable().scaledToFill()
        } else {
            RoundedRectangle(cornerRadius: 14).fill(KinoTheme.raised).skeletonShimmer()
        }
    }
}
