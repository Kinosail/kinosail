import SwiftUI

struct FeaturePlaceholder: View {
    let title: String
    let symbol: String
    let message: String

    var body: some View {
        ContentUnavailableView {
            Label(title, systemImage: symbol)
        } description: {
            Text(message)
        }
        .frame(maxWidth: .infinity, minHeight: 220)
        .foregroundStyle(KinoTheme.text)
        .accessibilityElement(children: .combine)
    }
}
