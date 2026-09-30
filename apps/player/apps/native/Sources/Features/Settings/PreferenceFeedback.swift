import SwiftUI

struct PreferenceFeedback: View {
    let busy: Bool
    let message: String?
    let offersRetry: Bool
    let retry: () -> Void

    var body: some View {
        Group {
            if !busy, let message {
                Section {
                    Text(message).foregroundStyle(KinoTheme.muted)
                    if offersRetry { Button("Try again", action: retry) }
                }
            }
        }
        #if os(iOS)
        .toolbar { if busy { ToolbarItem(placement: .topBarTrailing) { ProgressView().accessibilityLabel("Saving preferences") } } }
        #endif
    }
}
