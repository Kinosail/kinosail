import SwiftUI

struct FeaturePlaceholder: View {
    let title: String
    let symbol: String
    let message: String

    var body: some View {
        ContentUnavailableView {
            Label(title, systemImage: symbol)
        } description: {
            Text(message).foregroundStyle(KinoTheme.muted)
                .frame(maxWidth: 560).fixedSize(horizontal: false, vertical: true)
        }
        .frame(maxWidth: .infinity, minHeight: 220)
        .foregroundStyle(KinoTheme.text)
        .accessibilityElement(children: .combine)
    }
}
