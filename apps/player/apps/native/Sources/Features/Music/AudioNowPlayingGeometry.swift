import SwiftUI

enum AudioNowPlayingGeometry {
    static var transportSpacing: CGFloat {
        #if os(tvOS)
        28
        #else
        12
        #endif
    }
    static var trackSpacing: CGFloat {
        #if os(tvOS)
        transportSpacing * 2 + transportSize(1)
        #else
        24
        #endif
    }
    static func transportSize(_ index: Int) -> CGFloat {
        #if os(tvOS)
        index == 1 ? 112 : 128
        #else
        52
        #endif
    }
    static func width(accessibility: Bool) -> CGFloat {
        #if os(tvOS)
        accessibility ? 900 : 1400
        #else
        700
        #endif
    }
    static func layout(accessibility: Bool) -> AnyLayout {
        #if os(tvOS)
        accessibility ? AnyLayout(VStackLayout(spacing: 28))
                      : AnyLayout(HStackLayout(alignment: .top, spacing: 64))
        #else
        AnyLayout(VStackLayout(spacing: 28))
        #endif
    }
}

