#if os(tvOS)
import ObjectiveC
import OSLog
import UIKit

@MainActor enum RemoteTouchDelays {
    private static let log = Logger(subsystem: "com.kinosail.player", category: "remote-touch")
    private static var guardedImplementation: IMP?

    static func install() {
        guard guardedImplementation == nil,
              let original = class_getInstanceMethod(UIApplication.self, #selector(UIApplication.sendEvent(_:))),
              let guarded = class_getInstanceMethod(UIApplication.self, #selector(UIApplication.kinosailGuardedSendEvent(_:))) else { return }
        guardedImplementation = method_getImplementation(guarded)
        method_exchangeImplementations(original, guarded)
    }

    static var guardsEvents: Bool {
        guard let guardedImplementation, let current = class_getInstanceMethod(UIApplication.self, #selector(UIApplication.sendEvent(_:))) else { return false }
        return method_getImplementation(current) == guardedImplementation
    }

    static func startsGesture(_ phases: [UITouch.Phase]) -> Bool {
        !phases.isEmpty && phases.allSatisfy { $0 == .began }
    }

    static func prepare(for event: UIEvent, in application: UIApplication) {
        guard event.type == .touches, let touches = event.allTouches, startsGesture(touches.map(\.phase)) else { return }
        release(in: application.connectedScenes.flatMap { ($0 as? UIWindowScene)?.windows ?? [] })
    }

    static func release(in views: [UIView]) {
        let released = views.reduce(0) { $0 + release(view: $1) }
        if released > 0 { log.debug("operation=remote-touch-delays outcome=released count=\(released)") }
    }

    private static func release(view: UIView) -> Int {
        var released = 0
        for recognizer in view.gestureRecognizers ?? [] where recognizer.delaysTouchesBegan || recognizer.delaysTouchesEnded {
            recognizer.delaysTouchesBegan = false
            recognizer.delaysTouchesEnded = false
            released += 1
        }
        return view.subviews.reduce(released) { $0 + release(view: $1) }
    }
}

extension UIApplication {
    @objc fileprivate func kinosailGuardedSendEvent(_ event: UIEvent) {
        RemoteTouchDelays.prepare(for: event, in: self)
        kinosailGuardedSendEvent(event)
    }
}
#endif
