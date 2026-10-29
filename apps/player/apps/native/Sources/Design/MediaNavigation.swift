import SwiftUI

extension MediaItem {
    func destination(inShows: Bool) -> ScreenDestination {
        #if os(tvOS)
        if kind == .photo { return .photos(id) }
        #endif
        return inShows && !showID.isEmpty ? .show(showID) : destination
    }
    var destination: ScreenDestination {
        kind == .show ? .show(showID.isEmpty ? id : showID) : .detail(id)
    }
    var playingDestination: ScreenDestination {
        switch kind {
        case .photo: .photos(id)
        case .book: .reader(id)
        case .show: .show(showID.isEmpty ? id : showID)
        case .music, .audiobook: .audio(id)
        case .video: .playback(id)
        }
    }
}

extension Double {
    var clock: String {
        guard isFinite, self >= 0, self <= 31_536_000 else { return "0:00" }
        let seconds = Int(self)
        return seconds >= 3600 ? String(format: "%d:%02d:%02d", seconds / 3600, seconds / 60 % 60, seconds % 60)
            : String(format: "%d:%02d", seconds / 60, seconds % 60)
    }
}

extension View {
    @ViewBuilder func mediaLineLimit(_ lines: Int, accessibility: Bool) -> some View {
        if accessibility { lineLimit(nil) }
        else { lineLimit(lines, reservesSpace: true) }
    }
}
