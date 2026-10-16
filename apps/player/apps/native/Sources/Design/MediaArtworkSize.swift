import SwiftUI

enum MediaArtworkSize {
    static func dimension(landscape: Bool, accessibility: Bool, displayScale: CGFloat) -> Int {
        if accessibility { return 1600 }
        #if os(tvOS)
        if landscape { return 800 }
        return displayScale > 1 ? 500 : 400
        #else
        return landscape ? 1600 : 800
        #endif
    }
}
