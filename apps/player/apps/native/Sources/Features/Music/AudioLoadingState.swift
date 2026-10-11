import SwiftUI

struct AudioLoadingState: View {
    var title = "Opening audio…"
    var showsTrackControls = false
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    private var nowPlayingLayout: AnyLayout { AudioNowPlayingGeometry.layout(accessibility: dynamicTypeSize.isAccessibilitySize) }
    private var optionWidth: CGFloat {
        #if os(tvOS)
        264
        #else
        94
        #endif
    }
    private var optionHeight: CGFloat {
        #if os(tvOS)
        68
        #else
        44
        #endif
    }
    var body: some View {
        nowPlayingLayout {
            RoundedRectangle(cornerRadius: 24).fill(KinoTheme.surface).aspectRatio(1, contentMode: .fit).frame(maxWidth: 440)
            VStack(spacing: 28) {
                VStack(spacing: 8) {
                    #if os(tvOS)
                    Text(verbatim: "Title").font(.title.bold()).hidden().frame(width: 260)
                        .overlay { RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised) }
                    Text(verbatim: "Artist").hidden().frame(width: 160)
                        .overlay { RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised) }
                    #else
                    RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(width: 260, height: 36)
                    RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(width: 160, height: 16)
                    #endif
                }
                VStack(spacing: 8) {
                    Capsule().fill(KinoTheme.raised).frame(height: 5)
                    #if os(tvOS)
                    HStack {
                        Text(verbatim: "0:00"); Spacer(); Text(verbatim: "0:00")
                    }.font(.caption.monospacedDigit()).hidden()
                    #endif
                }
                HStack(spacing: AudioNowPlayingGeometry.transportSpacing) {
                    ForEach(0..<3) { index in Circle().fill(KinoTheme.surface)
                        .frame(width: AudioNowPlayingGeometry.transportSize(index), height: AudioNowPlayingGeometry.transportSize(index)) }
                }
                #if os(tvOS)
                if showsTrackControls {
                    HStack(spacing: AudioNowPlayingGeometry.trackSpacing) {
                        ForEach(0..<2) { _ in Capsule().fill(KinoTheme.surface).frame(width: 128, height: 92) }
                    }
                }
                #endif
                ViewThatFits(in: .horizontal) {
                    HStack(spacing: 12) {
                        ForEach(0..<3) { _ in Capsule().fill(KinoTheme.surface).frame(width: optionWidth, height: optionHeight) }
                    }
                    VStack(spacing: 12) {
                        ForEach(0..<3) { _ in Capsule().fill(KinoTheme.surface).frame(width: optionWidth, height: optionHeight) }
                    }
                }
            }
        }
        .frame(maxWidth: AudioNowPlayingGeometry.width(accessibility: dynamicTypeSize.isAccessibilitySize)).frame(maxWidth: .infinity)
        .skeletonLoading(title)
    }
}

