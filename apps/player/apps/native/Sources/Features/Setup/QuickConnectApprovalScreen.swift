import SwiftUI

struct QuickConnectApprovalScreen: View {
    let link: QuickConnectLink
    let done: () -> Void
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var accounts: [SourceAccount] = []
    @State private var chosen: String?
    @State private var loaded = false
    @State private var working = false
    @State private var approved = false
    @State private var failure: String?

    private var account: SourceAccount? { accounts.first { $0.id == chosen } ?? accounts.first }

    var body: some View {
        ScrollView {
            VStack(spacing: 24) {
                if !loaded {
                    ProgressView().controlSize(.large)
                } else if approved {
                    success
                } else if let account {
                    request(account)
                } else {
                    missing
                }
            }
            .padding(.horizontal, KinoTheme.contentPadding)
            .padding(.vertical, 32)
            .frame(maxWidth: 520)
            .frame(maxWidth: .infinity)
            .animation(reduceMotion ? .easeInOut(duration: 0.2) : .spring(duration: 0.45, bounce: 0.2), value: "\(loaded)\(approved)")
        }
        .defaultScrollAnchor(.center, for: .alignment)
        .cinemaBackground()
        .toolbar { ToolbarItem(placement: .cancellationAction) { Button(approved ? "Done" : "Cancel", action: done) } }
        #if os(iOS)
        .sensoryFeedback(.success, trigger: approved)
        #endif
        .task { await load() }
    }

    private func request(_ account: SourceAccount) -> some View {
        VStack(spacing: 20) {
            Image(systemName: "tv").font(.system(size: 54, weight: .semibold)).foregroundStyle(KinoTheme.signal).accessibilityHidden(true)
            Text("Sign in your TV?").font(.system(.largeTitle, design: .rounded).bold()).foregroundStyle(KinoTheme.text)
                .accessibilityAddTraits(.isHeader)
            Text("Check that your TV shows this code.").font(.body).foregroundStyle(KinoTheme.muted)
            Text(link.code).font(.system(size: 44, weight: .bold, design: .rounded).monospacedDigit()).tracking(8)
                .foregroundStyle(KinoTheme.text)
                .accessibilityLabel("Code \(link.code.map(String.init).joined(separator: " "))")
            VStack(spacing: 10) {
                ForEach(accounts) { option in
                    Button { chosen = option.id } label: {
                        ServerCard(brand: .source(option.kind), name: option.name, detail: "Signed in as \(option.userName)", connected: option.id == account.id)
                    }
                    .serverCardButtonStyle()
                    .accessibilityAddTraits(option.id == account.id ? .isSelected : [])
                }
            }
            Button { approve(account) } label: {
                Text("Approve").font(.headline).foregroundStyle(KinoTheme.actionInk)
                    .opacity(working ? 0 : 1)
                    .overlay { if working { ProgressView().tint(KinoTheme.signalInk) } }
                    .frame(maxWidth: .infinity, minHeight: 44)
            }
            .primaryCapsule()
            .disabled(working)
            .accessibilityLabel(working ? "Approving" : "Approve")
            if let failure { Label(failure, systemImage: "exclamationmark.triangle.fill").font(.callout).foregroundStyle(.red) }
        }
    }

    private var success: some View {
        VStack(spacing: 16) {
            Image(systemName: "checkmark.circle.fill").font(.system(size: 64)).foregroundStyle(KinoTheme.signal)
                .symbolEffect(.bounce, value: approved).accessibilityHidden(true)
            Text("Your TV is signed in").font(.system(.title, design: .rounded).bold()).foregroundStyle(KinoTheme.text)
                .accessibilityAddTraits(.isHeader)
            Text("It opens your library in a moment.").font(.body).foregroundStyle(KinoTheme.muted)
        }
    }

    private var missing: some View {
        VStack(spacing: 14) {
            Image(systemName: "person.crop.circle.badge.questionmark").font(.system(size: 54)).foregroundStyle(KinoTheme.muted)
                .accessibilityHidden(true)
            Text("Sign in on this iPhone first").font(.title2.bold()).foregroundStyle(KinoTheme.text).accessibilityAddTraits(.isHeader)
            Text("This iPhone is not signed in to the Jellyfin server your TV shows. Add that server here, then scan the code again.")
                .font(.body).foregroundStyle(KinoTheme.muted).multilineTextAlignment(.center)
        }
    }

    private func load() async {
        let local = (try? await SourceAccountStore().load()) ?? []
        let shared = await SourceSharedStore.shared.load()
        var seen = Set<String>()
        accounts = (local + shared).filter { $0.kind == .jellyfin && $0.serverID == link.serverID && seen.insert($0.id).inserted }
        loaded = true
    }

    private func approve(_ account: SourceAccount) {
        guard !working else { return }
        working = true
        failure = nil
        Task {
            defer { working = false }
            do {
                try await SourceConnect.approve(code: link.code, account: account)
                approved = true
            } catch { failure = AppSession.message(error) }
        }
    }
}
