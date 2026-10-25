#if os(iOS)
import SwiftUI

struct TouchPlaybackTransport: View {
    let preparing: Bool
    let disabled: Bool
    let back: () -> Void
    let toggle: () -> Void
    let forward: () -> Void
    @Environment(AppSession.self) private var session

    private var playback: PlaybackCoordinator { session.player }
    private var playing: Bool { playback.isPlaying || playback.buffering }
    private var symbol: String { playback.completed ? "arrow.counterclockwise" : playing ? "pause.fill" : "play.fill" }

    var body: some View {
        HStack(spacing: 40) {
            Button("Back 10 seconds", systemImage: "gobackward.10", action: back)
                .font(.title2.weight(.semibold)).frame(width: 60, height: 60)
                .glassEffect(.regular.interactive(), in: .circle)
                .keyboardShortcut(.leftArrow, modifiers: [])
            Group {
                if preparing {
                    ProgressView().controlSize(.large).tint(.white).accessibilityHidden(true)
                } else {
                    Button(action: toggle) {
                        Image(systemName: symbol)
                            .font(.system(size: 34, weight: .bold))
                            .contentTransition(.symbolEffect(.replace))
                            .frame(width: 84, height: 84)
                    }
                    .accessibilityLabel(playback.completed ? "Replay" : playing ? "Pause" : "Play")
                    .keyboardShortcut(.space, modifiers: [])
                }
            }
            .frame(width: 84, height: 84)
            .glassEffect(.regular.interactive(), in: .circle)
            Button("Forward 10 seconds", systemImage: "goforward.10", action: forward)
                .font(.title2.weight(.semibold)).frame(width: 60, height: 60)
                .glassEffect(.regular.interactive(), in: .circle)
                .keyboardShortcut(.rightArrow, modifiers: [])
        }
        .labelStyle(.iconOnly).buttonStyle(.plain)
        .disabled(disabled)
    }
}
#endif
