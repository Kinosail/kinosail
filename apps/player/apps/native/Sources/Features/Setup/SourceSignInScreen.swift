import SwiftUI

struct NearbySourceRow: View {
    let server: SourceServerInfo

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: "play.rectangle.on.rectangle").foregroundStyle(KinoTheme.signal).accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 4) {
                Text(server.name)
                Text("\(server.kind.title) · \(server.address.host ?? "")").font(.caption).foregroundStyle(.secondary)
            }
        }
        .accessibilityElement(children: .combine)
    }
}

struct SourceSignInScreen: View {
    let server: SourceServerInfo
    @Environment(AppSession.self) private var session
    @State private var user = ""
    @State private var password = ""
    @State private var working = false
    @State private var failure: String?
    @State private var quickConnect: SourceQuickConnect?
    @State private var quickConnectAvailable = false
    @State private var codeRevision = 0
    @FocusState private var passwordFocused: Bool

    var body: some View {
        Form {
            Section {
                VStack(alignment: .leading, spacing: 6) {
                    Text(server.name).font(.title2.bold()).accessibilityAddTraits(.isHeader)
                    Text("\(server.kind.title) \(server.version) · \(server.address.host ?? "")").font(.subheadline).foregroundStyle(KinoTheme.muted)
                }
                .padding(.vertical, 6)
            }
            if quickConnectAvailable {
                Section {
                    if let quickConnect {
                        Text(quickConnect.code)
                            .font(.system(.largeTitle, design: .monospaced).weight(.semibold))
                            .frame(maxWidth: .infinity)
                            .accessibilityLabel("Quick Connect code \(quickConnect.code.map(String.init).joined(separator: " "))")
                        Text("Waiting for approval…").font(.callout).foregroundStyle(.secondary)
                    } else {
                        ProgressView().frame(maxWidth: .infinity)
                    }
                } header: { Text("Quick Connect") } footer: {
                    Text("On a phone or computer signed in to Jellyfin, open your profile, choose Quick Connect and enter this code.")
                }
            }
            Section {
                TextField("User name", text: $user)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .textContentType(.username)
                    .submitLabel(.next)
                    .onSubmit { passwordFocused = true }
                SecureField("Password", text: $password)
                    .textContentType(.password)
                    .submitLabel(.go)
                    .focused($passwordFocused)
                    .onSubmit { signIn() }
                Button(working ? "Signing in…" : "Sign in") { signIn() }
                    .disabled(working || user.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            } header: { Text(quickConnectAvailable ? "Or sign in with a password" : "Sign in") } footer: {
                if let failure { Text(failure).foregroundStyle(.red) }
                else { Text("Use the account you use in \(server.kind.title). Kinosail stores its sign-in securely on this device.") }
            }
        }
        #if os(iOS)
        .scrollContentBackground(.hidden)
        .frame(maxWidth: 600)
        .frame(maxWidth: .infinity)
        .cinemaBackground()
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .navigationTitle(server.kind.title)
        .task { quickConnectAvailable = await SourceConnect.quickConnectEnabled(server) }
        .task(id: "\(quickConnectAvailable)-\(codeRevision)") { await runQuickConnect() }
    }

    private func runQuickConnect() async {
        guard quickConnectAvailable else { return }
        do {
            let pending = try await SourceConnect.startQuickConnect(server)
            quickConnect = pending
            let account = try await SourceConnect.awaitQuickConnect(server, pending, until: Date().addingTimeInterval(270))
            try await finish(account)
        } catch is CancellationError {
        } catch ClientError.invalidInput {
            quickConnect = nil
            codeRevision += 1
        } catch {
            quickConnect = nil
            failure = AppSession.message(error)
        }
    }

    private func signIn() {
        guard !working else { return }
        working = true
        failure = nil
        Task {
            defer { working = false }
            do { try await finish(SourceConnect.signIn(server, user: user, password: password)) }
            catch { failure = AppSession.message(error) }
        }
    }

    private func finish(_ account: SourceAccount) async throws {
        password = ""
        try await session.connectLibrary(account)
    }
}
