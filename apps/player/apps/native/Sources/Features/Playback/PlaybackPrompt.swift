import SwiftUI

struct PlaybackPrompt: Equatable {
    enum Action: Equatable { case skip(to: Double), next(String) }
    let action: Action
    let title: String
    let symbol: String
}

extension PlaybackCoordinator {
    private static let skippable: Set<String> = ["intro", "recap", "commercial"]

    var prompt: PlaybackPrompt? {
        guard let source, currentItem?.kind == .video, duration > 0, message == nil else { return nil }
        let now = seconds
        if !completed, let marker = source.markers.first(where: {
            Self.skippable.contains($0.type) && !source.autoSkip.contains($0.type) && now >= $0.start && now < $0.end - 1
        }) {
            let title = switch marker.type { case "intro": "Skip Intro"; case "recap": "Skip Recap"; default: "Skip" }
            return PlaybackPrompt(action: .skip(to: marker.end), title: title, symbol: "forward.end.fill")
        }
        guard let next = source.nextItemID else { return nil }
        let ending = source.markers.filter { ($0.type == "credits" || $0.type == "outro") && $0.start > duration / 2 }
            .map(\.start).min() ?? max(duration / 2, duration - 30)
        guard completed || now >= ending else { return nil }
        let remaining = Int((duration - now).rounded(.up))
        let title = !completed && remaining <= 10 ? "Next Episode in \(remaining)" : "Next Episode"
        return PlaybackPrompt(action: .next(next), title: title, symbol: "forward.fill")
    }
}

extension AppSession {
    func perform(_ prompt: PlaybackPrompt) async throws {
        switch prompt.action {
        case .skip(let end): try await player.seek(to: end)
        case .next(let id):
            guard let client, let store = progress else { throw ClientError.unavailable }
            try await player.play(client.item(id: id), client: client, store: store)
        }
    }
}

#if os(iOS)
struct PlaybackPromptButton: View {
    let prompt: PlaybackPrompt
    let bottom: CGFloat
    @Environment(AppSession.self) private var session
    @State private var tapped = 0

    var body: some View {
        Button {
            tapped += 1
            Task { try? await session.perform(prompt) }
        } label: {
            Label(prompt.title, systemImage: prompt.symbol)
                .font(.callout.weight(.semibold)).monospacedDigit()
                .padding(.horizontal, 18).padding(.vertical, 12)
                .foregroundStyle(.black)
                .background(.white, in: Capsule())
                .shadow(color: .black.opacity(0.35), radius: 12, y: 4)
        }
        .buttonStyle(.plain)
        .contentTransition(.numericText())
        .sensoryFeedback(.impact(weight: .light), trigger: tapped)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottomTrailing)
        .padding(.trailing, 24)
        .padding(.bottom, bottom)
        .transition(.move(edge: .trailing).combined(with: .opacity))
    }
}
#endif

#if os(tvOS)
struct TVPlaybackPromptActions: View {
    @Environment(AppSession.self) private var session

    var body: some View {
        Color.clear
            .allowsHitTesting(false)
            .accessibilityHidden(true)
            .onChange(of: session.player.prompt, initial: true) { _, prompt in apply(prompt) }
            .onDisappear { session.player.presentation.controller.contextualActions = [] }
    }

    private func apply(_ prompt: PlaybackPrompt?) {
        let controller = session.player.presentation.controller
        guard controller.contextualActions.map(\.title) != (prompt.map { [$0.title] } ?? []) else { return }
        controller.contextualActions = prompt.map { prompt in
            [UIAction(title: prompt.title, image: UIImage(systemName: prompt.symbol)) { _ in
                Task { try? await session.perform(prompt) }
            }]
        } ?? []
    }
}
#endif
