#if os(iOS)
import SwiftUI

extension GeometryProxy {
    var playbackDivision: CGRect? {
        if #available(iOS 27.1, *) { return reservedRegions(kind: .division).first?.frame }
        return nil
    }
}

enum TouchPlaybackLayout {
    static func controlArea(size: CGSize, division: CGRect?) -> CGRect {
        let full = CGRect(origin: .zero, size: size)
        guard let division, !division.isEmpty, division.intersects(full) else { return full }
        if division.height > division.width {
            let left = CGRect(x: 0, y: 0, width: max(0, division.minX), height: size.height)
            let right = CGRect(x: min(size.width, division.maxX), y: 0,
                               width: max(0, size.width - division.maxX), height: size.height)
            return left.width >= right.width ? left : right
        }
        let top = CGRect(x: 0, y: 0, width: size.width, height: max(0, division.minY))
        let bottom = CGRect(x: 0, y: min(size.height, division.maxY), width: size.width,
                            height: max(0, size.height - division.maxY))
        return bottom.height >= top.height ? bottom : top
    }
}
#endif
