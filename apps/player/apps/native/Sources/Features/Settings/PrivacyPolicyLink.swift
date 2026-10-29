import SwiftUI

struct PrivacyPolicyLink: View {
    var symbol: String?
    private let url = URL(string: "https://kinosail.com/privacy/")!

    var body: some View {
        #if os(tvOS)
        NavigationLink {
            ScrollView {
                VStack(spacing: 24) {
                    Text("Your library stays on your Server. Kinosail Player stores your session on this device and connects directly to the Server you choose.")
                        .frame(maxWidth: 700)
                    QRCodeView(value: url.absoluteString).frame(width: 224, height: 224)
                        .accessibilityLabel("Scan this code to read the privacy policy")
                    Text("Scan to read the full privacy policy, including storage and deletion: \(url.absoluteString)")
                        .frame(maxWidth: 700)
                }
                .multilineTextAlignment(.center)
                .frame(maxWidth: .infinity)
                .padding(KinoTheme.contentPadding)
            }
            .navigationTitle("Privacy policy")
        } label: { label }
        #else
        Link(destination: url) { label }
        #endif
    }

    @ViewBuilder private var label: some View {
        if let symbol { Label("Privacy policy", systemImage: symbol) } else { Text("Privacy policy") }
    }
}
