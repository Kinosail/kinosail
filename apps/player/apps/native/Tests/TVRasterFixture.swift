#if os(tvOS)
import SwiftUI
import UIKit

@MainActor
func tvRasterFixture<Content: View>(_ content: Content, width: CGFloat) async throws -> (image: CGImage, pixels: Data) {
    guard let scene = UIApplication.shared.connectedScenes.first as? UIWindowScene else { throw TVRasterFailure.scene }
    let previousKey = scene.windows.first { $0.isKeyWindow }
    let window = UIWindow(windowScene: scene)
    let controller = UIHostingController(rootView: content)
    controller.safeAreaRegions = []
    controller.view.backgroundColor = .clear
    controller.view.isOpaque = false
    window.backgroundColor = .clear
    window.rootViewController = controller
    defer {
        window.isHidden = true
        window.rootViewController = nil
        if let previousKey, !previousKey.isKeyWindow { previousKey.makeKey() }
    }
    let fitted = controller.sizeThatFits(in: CGSize(width: width, height: 4096))
    guard fitted.width.isFinite, fitted.height.isFinite,
          (1...4096).contains(fitted.width), (1...4096).contains(fitted.height) else { throw TVRasterFailure.size }
    let size = CGSize(width: ceil(fitted.width), height: ceil(fitted.height))
    let bounds = CGRect(origin: .zero, size: size)
    window.frame = bounds
    controller.view.frame = bounds
    window.isHidden = false
    controller.view.setNeedsLayout()
    controller.view.layoutIfNeeded()
    try await Task.sleep(for: .milliseconds(100))
    controller.view.layoutIfNeeded()
    guard !window.isKeyWindow, previousKey.map({ $0.isKeyWindow }) ?? true else { throw TVRasterFailure.focus }
    let format = UIGraphicsImageRendererFormat()
    format.scale = 1; format.opaque = false
    var drawn = false
    let painted = UIGraphicsImageRenderer(size: size, format: format).image { _ in
        drawn = controller.view.drawHierarchy(in: bounds, afterScreenUpdates: true)
    }
    guard drawn, let image = painted.cgImage else { throw TVRasterFailure.drawing }
    let columns = Int(size.width), rows = Int(size.height), rowBytes = Int(size.width) * 4
    guard let context = CGContext(data: nil, width: columns, height: rows, bitsPerComponent: 8,
        bytesPerRow: rowBytes, space: CGColorSpaceCreateDeviceRGB(),
        bitmapInfo: CGBitmapInfo.byteOrder32Big.rawValue | CGImageAlphaInfo.premultipliedLast.rawValue),
        let storage = context.data else { throw TVRasterFailure.bitmap }
    context.clear(bounds); context.draw(image, in: bounds)
    let pixels = Data(bytes: storage, count: rowBytes * rows)
    guard stride(from: 3, to: pixels.count, by: 4).contains(where: { pixels[$0] != 0 }),
          let captured = context.makeImage() else { throw TVRasterFailure.blank }
    return (captured, pixels)
}

private enum TVRasterFailure: Error { case scene, size, focus, drawing, bitmap, blank }
#endif
