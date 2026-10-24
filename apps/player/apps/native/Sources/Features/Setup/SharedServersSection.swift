import SwiftUI

struct SharedServersSection: View {
    let shared: [SourceAccount]
    let excluded: Set<String>
    let ready: (SourceAccount) -> Void
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var adopting: String?
    @State private var failure: String?

    private var visible: [SourceAccount] { shared.filter { !excluded.contains($0.id) } }

    var body: some View {
        Group {
            if !visible.isEmpty {
                VStack(alignment: .leading, spacing: 14) {
                    Label("From your Apple devices", systemImage: "icloud.fill")
                        .font(.title3.bold()).foregroundStyle(KinoTheme.text)
                        .labelStyle(SharedTitleStyle())
                        .accessibilityAddTraits(.isHeader)
                    VStack(spacing: 12) {
                        ForEach(visible) { account in
                            Button { adopt(account) } label: {
                                ServerCard(brand: .source(account.kind), name: account.name,
                                           detail: account.name == account.kind.title ? "Signed in as \(account.userName)" : "\(account.kind.title) · \(account.userName)", badge: "Ready · no sign-in needed")
                                    .overlay(alignment: .trailing) {
                                        if adopting == account.id { ProgressView().padding(.trailing, 18) }
                                    }
                            }
                            .serverCardButtonStyle()
                            .disabled(adopting != nil)
                            .cardAppearance(reduceMotion: reduceMotion)
                            .accessibilityLabel("\(account.name), \(account.kind.title), signed in as \(account.userName)")
                            .accessibilityHint("Adds this server without signing in again")
                        }
                    }
                    if let failure {
                        Label(failure, systemImage: "exclamationmark.triangle.fill").font(.callout).foregroundStyle(.red)
                    }
                }
                .transition(.opacity)
            }
        }
        .animation(reduceMotion ? .easeInOut(duration: 0.2) : .spring(duration: 0.5, bounce: 0.2), value: visible.map(\.id))
    }

    private func adopt(_ account: SourceAccount) {
        guard adopting == nil else { return }
        adopting = account.id
        failure = nil
        Task {
            defer { adopting = nil }
            do { ready(try await SourceConnect.adopt(account)) }
            catch is CancellationError {}
            catch { failure = AppSession.message(error) }
        }
    }
}

private struct SharedTitleStyle: LabelStyle {
    func makeBody(configuration: Configuration) -> some View {
        HStack(spacing: 10) {
            configuration.icon.font(.body.weight(.semibold)).foregroundStyle(KinoTheme.signal)
            configuration.title
        }
    }
}
