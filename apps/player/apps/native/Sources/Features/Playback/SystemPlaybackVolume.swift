#if os(iOS)
import MediaPlayer
import SwiftUI

struct SystemPlaybackVolume: UIViewRepresentable {
    func makeUIView(context: Context) -> MPVolumeView { MPVolumeView() }
    func updateUIView(_ view: MPVolumeView, context: Context) {}
}
#endif
