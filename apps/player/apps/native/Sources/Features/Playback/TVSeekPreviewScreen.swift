#if os(tvOS)
import SwiftUI

struct TVSeekPreviewScreen: View {
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @FocusState private var seekFocused: Bool
    @State private var position = 0.0
    @State private var image: UIImage?
    @State private var message: String?

    private var duration: Double { max(1, session.player.duration) }
    private var frameSecond: Int { Int(min(43210, max(0, position)) / 10) * 10 }

    var body: some View {
        VStack(spacing: 20) {
            Text("Seek with preview").font(.title2.bold())
            if let image {
                Image(uiImage: image).resizable().aspectRatio(contentMode: .fit)
                    .frame(width: 480, height: 270).background(.black)
                    .clipShape(RoundedRectangle(cornerRadius: 12))
            } else {
                ZStack {
                    Color.black
                    if let message { Text(message).font(.body).foregroundStyle(.white) }
                    else { ProgressView() }
                }
                .frame(width: 480, height: 270)
                .clipShape(RoundedRectangle(cornerRadius: 12))
            }
            Text(position.clock).font(.headline.monospacedDigit())
            Button(action: seek) {
                VStack(spacing: 8) {
                    ProgressView(value: position, total: duration)
                    Text("Left or right to preview · Select to seek").font(.caption)
                }
            }
                .buttonStyle(.plain)
                .focused($seekFocused)
                .onMoveCommand { direction in
                    if direction == .left { position = max(0, position - 10) }
                    if direction == .right { position = min(duration, position + 10) }
                }
                .accessibilityLabel("Playback position")
                .accessibilityValue("\(position.clock) of \(duration.clock)")
            HStack(spacing: 24) {
                Button("Cancel") { dismiss() }
                Button("Seek here", action: seek)
                .disabled(session.player.duration <= 0)
            }
        }
        .padding(48)
        .frame(width: 720)
        .background(KinoTheme.surface, in: RoundedRectangle(cornerRadius: 24))
        .onAppear { position = min(duration, max(0, session.player.seconds)); seekFocused = true }
        .task(id: frameSecond) {
            image = nil; message = nil
            do {
                let preview = try await PlaybackFramePreview.load(template: session.player.source?.trickplay,
                                                                 client: session.client, asset: session.player.player?.currentItem?.asset,
                                                                 second: frameSecond)
                try Task.checkCancellation()
                image = preview
            } catch is CancellationError {} catch { if !Task.isCancelled { message = "Preview unavailable" } }
        }
    }

    private func seek() {
        Task {
            do { try await session.player.seek(to: position); dismiss() }
            catch { message = AppSession.message(error) }
        }
    }
}
#endif
