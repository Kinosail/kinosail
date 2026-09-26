#if os(iOS)
import SwiftUI

struct TouchPictureInPictureView: View {
    let stop: () -> Void

    var body: some View {
        ContentUnavailableView {
            Label("Playing in Picture in Picture", systemImage: "pip")
        } description: {
            Text("Your video is in a floating window.")
        } actions: {
            Button("Return to video", action: stop)
                .buttonStyle(.borderedProminent).buttonBorderShape(.capsule)
                .tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
        }
    }
}
#endif
