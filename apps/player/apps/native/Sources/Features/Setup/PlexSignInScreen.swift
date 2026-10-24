import SwiftUI
#if os(iOS)
import AuthenticationServices
#endif

struct PlexSignInScreen: View {
    let target: String?
    let signedIn: (SourceAccount) -> Void
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    #if os(iOS)
    @Environment(\.webAuthenticationSession) private var webAuthentication
    #endif
    @State private var pin: PlexPin?
    @State private var pinRevision = 0
    @State private var web: Task<Void, Never>?
    @State private var servers: [PlexServer] = []
    @State private var user: (id: String, name: String)?
    @State private var connecting: String?
    @State private var failure: String?
    @MainActor private static var remembered: (token: String, user: (id: String, name: String), servers: [PlexServer], at: Date)?

    var body: some View {
        layout
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            .sensoryFeedback(.error, trigger: failure) { _, new in new != nil }
            #endif
            .animation(reduceMotion ? .easeInOut(duration: 0.2) : .spring(duration: 0.45, bounce: 0.18), value: "\(pin?.code ?? "")\(servers.count)")
            .cinemaBackground()
            .navigationTitle("")
            .task(id: pinRevision) { await run() }
            .onDisappear { web?.cancel() }
    }

    @ViewBuilder private var layout: some View {
        #if os(tvOS)
        HStack(alignment: .center, spacing: 96) {
            header.frame(width: 640, alignment: .leading)
            VStack(spacing: 28) { content; failureLabel }.frame(width: 720).focusSection()
        }
        .padding(.horizontal, KinoTheme.contentPadding)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        #else
        ScrollView {
            VStack(spacing: 32) { header; content; failureLabel }
                .padding(.horizontal, KinoTheme.contentPadding)
                .padding(.vertical, 24)
                .frame(maxWidth: 620)
                .frame(maxWidth: .infinity)
        }
        .defaultScrollAnchor(.center, for: .alignment)
        #endif
    }

    private var header: some View {
        VStack(alignment: tv ? .leading : .center, spacing: 14) {
            ServerMonogram(brand: .source(.plex), size: tv ? 96 : 72)
            Text(servers.isEmpty ? "Sign in with Plex" : "Choose a server").font(.system(.title, design: .rounded).bold())
                .foregroundStyle(KinoTheme.text).accessibilityAddTraits(.isHeader)
            Text(servers.isEmpty ? "Your Plex servers, shared libraries and watch history come with you." : "Signed in as \(user?.name ?? "you"). You can add more servers later.")
                .font(.subheadline).foregroundStyle(KinoTheme.muted).multilineTextAlignment(tv ? .leading : .center)
        }
        .frame(maxWidth: .infinity, alignment: tv ? .leading : .center)
    }

    @ViewBuilder private var content: some View {
        if !servers.isEmpty { serverList }
        else if let pin { codeCard(pin) }
        else { ProgressView().controlSize(.large).padding(.top, 24).accessibilityLabel("Contacting Plex") }
    }

    private func codeCard(_ pin: PlexPin) -> some View {
        VStack(spacing: 20) {
            #if os(iOS)
            Button { openWeb(pin) } label: {
                Label("Continue with Plex", systemImage: "person.badge.key.fill").font(.headline).foregroundStyle(KinoTheme.actionInk)
                    .frame(maxWidth: .infinity, minHeight: 44)
            }
            .primaryCapsule()
            Text("Or go to \(PlexAccounts.linkPage()) on any device and enter").font(.footnote).foregroundStyle(KinoTheme.muted)
            #endif
            HStack(spacing: 28) {
                VStack(spacing: 12) {
                    Text(pin.code)
                        .font(.system(size: tv ? 88 : 40, weight: .bold, design: .rounded).monospaced())
                        .tracking(tv ? 14 : 8)
                        .foregroundStyle(KinoTheme.text)
                        .minimumScaleFactor(0.6).lineLimit(1)
                        .accessibilityLabel("Plex code \(pin.code.map(String.init).joined(separator: " "))")
                    HStack(spacing: 8) {
                        ScanPulse(active: true)
                        Text(connecting == nil ? "Waiting for Plex…" : "Connecting…").font(.callout).foregroundStyle(KinoTheme.muted)
                    }
                }
                .frame(maxWidth: .infinity)
                #if os(tvOS)
                if let url = PlexAccounts.authURL(pin, forward: false) {
                    QRCodeView(value: url.absoluteString).frame(width: 220, height: 220)
                        .accessibilityLabel("Scan with your iPhone to sign in to Plex")
                }
                #endif
            }
            #if os(tvOS)
            Text("Scan the code with your iPhone camera, or go to \(PlexAccounts.linkPage()) on your phone or computer and enter this code.")
                .font(.footnote).foregroundStyle(KinoTheme.muted)
            #endif
        }
        .card()
    }

    private var serverList: some View {
        VStack(spacing: 12) {
            ForEach(servers) { server in
                Button { choose(server) } label: {
                    ServerCard(brand: .source(.plex), name: server.name, detail: server.owned ? "Your server" : "Shared with you",
                               badge: connecting == server.id ? "Connecting…" : nil)
                }
                .serverCardButtonStyle()
                .disabled(connecting != nil)
            }
        }
    }

    @ViewBuilder private var failureLabel: some View {
        if let failure {
            Label(failure, systemImage: "exclamationmark.triangle.fill")
                .font(.callout).foregroundStyle(.red)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    private var tv: Bool {
        #if os(tvOS)
        true
        #else
        false
        #endif
    }

    private func run() async {
        if let saved = Self.remembered, Date().timeIntervalSince(saved.at) < 600 {
            user = saved.user
            await offer(saved.servers)
            return
        }
        do {
            let created = try await PlexAccounts.startPin()
            pin = created
            let token = try await PlexAccounts.awaitToken(created, until: Date().addingTimeInterval(600))
            web?.cancel()
            connecting = ""
            let (person, found) = try await (PlexAccounts.user(token: token), PlexAccounts.servers(token: token))
            connecting = nil
            guard !found.isEmpty else { failure = "Your Plex account has no servers yet. Set up Plex Media Server, then try again."; return }
            Self.remembered = (token, person, found, Date())
            user = person
            await offer(found)
        } catch is CancellationError {
        } catch ClientError.invalidInput(let message) where message.hasPrefix("The code expired") {
            pinRevision += 1
        } catch {
            connecting = nil
            failure = AppSession.message(error)
        }
    }

    private func offer(_ found: [PlexServer]) async {
        if let match = found.first(where: { $0.id == target }) ?? (found.count == 1 ? found.first : nil) {
            pin = nil
            servers = [match]
            choose(match)
        } else {
            servers = found
        }
    }

    private func choose(_ server: PlexServer) {
        guard connecting == nil, let user else { return }
        connecting = server.id
        failure = nil
        Task {
            defer { connecting = nil }
            do { signedIn(try await PlexAccounts.account(server, user: user)) }
            catch { failure = AppSession.message(error) }
        }
    }

    #if os(iOS)
    private func openWeb(_ pin: PlexPin) {
        guard let url = PlexAccounts.authURL(pin, forward: true) else { return }
        web?.cancel()
        web = Task { _ = try? await webAuthentication.authenticate(using: url, callbackURLScheme: "kinosail", preferredBrowserSession: .shared) }
    }
    #endif
}

struct PlexEntryRow: View {
    let open: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Have a Plex account?").font(.headline).foregroundStyle(KinoTheme.text).accessibilityAddTraits(.isHeader)
            Button(action: open) {
                ServerCard(brand: .source(.plex), name: "Sign in with Plex", detail: "Add your servers and shared libraries")
            }
            .serverCardButtonStyle()
        }
    }
}
