import SwiftUI
import UIKit

enum KinoTheme {
    // Electric green identifies actions; artwork retains its own colors.
    // System controls own focus, Dynamic Type, contrast and Reduce Motion.
    static let background = adaptive(dark: 0x0B0D0B, light: 0xF4F8EF)
    static let surface = adaptive(dark: 0x151914, light: 0xFFFFFF)
    static let raised = adaptive(dark: 0x20271E, light: 0xE9EFE2)
    static let text = adaptive(dark: 0xF6F8F2, light: 0x162011)
    static let muted = adaptive(dark: 0xA0A79C, light: 0x5B6852, highDark: 0xDAE2D3, highLight: 0x39482E)
    static let signal = adaptive(dark: 0xC4FF47, light: 0x3C6100, highDark: 0xDAFF98, highLight: 0x304E00)
    static let signalInk = adaptive(dark: 0x142000, light: 0xFFFFFF)

    // tvOS uses dark label ink on focused bordered controls. A dark surface
    // tint makes that system focus state unreadable. Keep its fill light.
    static var secondaryControlTint: Color {
        #if os(tvOS)
        .white
        #else
        raised
        #endif
    }

    static var contentPadding: CGFloat {
        #if os(tvOS)
        64
        #else
        20
        #endif
    }

    private static func adaptive(dark: UInt32, light: UInt32, highDark: UInt32? = nil, highLight: UInt32? = nil) -> Color {
        Color(uiColor: UIColor { traits in
            let increased = traits.accessibilityContrast == .high
            let value = traits.userInterfaceStyle == .dark ? (increased ? highDark ?? dark : dark) : (increased ? highLight ?? light : light)
            return UIColor(red: CGFloat((value >> 16) & 255) / 255,
                           green: CGFloat((value >> 8) & 255) / 255,
                           blue: CGFloat(value & 255) / 255, alpha: 1)
        })
    }
}
