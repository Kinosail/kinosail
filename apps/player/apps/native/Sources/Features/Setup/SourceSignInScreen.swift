import SwiftUI

struct SourceSignInScreen: View {
    let server: SourceServerInfo
    let signedIn: (SourceAccount) -> Void
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var loaded = false
    @State private var users: [SourcePublicUser] = []
    @State private var avatars: [String: UIImage] = [:]
    @State private var selected: SourcePublicUser?
    @State private var manual = false
    @State private var user = ""
    @State private var password = ""
    @State private var working = false
    @State private var failure: String?
    @State private var quickConnect: SourceQuickConnect?
    @State private var quickConnectAvailable = false
    @State private var codeRevision = 0
    @FocusState private var focus: Field?

    private enum Field { case user, password }

    var body: some View {
        layout
            #if os(iOS)
            .scrollDismissesKeyboard(.interactively)
            .navigationBarTitleDisplayMode(.inline)
            .sensoryFeedback(.error, trigger: failure) { _, new in new != nil }
            #endif
            .animation(reduceMotion ? .easeInOut(duration: 0.2) : .spring(duration: 0.45, bounce: 0.18), value: "\(loaded)\(selected?.id ?? "")\(manual)\(quickConnectAvailable)")
            .cinemaBackground()
            .navigationTitle("")
            .task { await load() }
            .task(id: "\(quickConnectAvailable)-\(codeRevision)") { await runQuickConnect() }
    }

    @ViewBuilder private var layout: some View {
        #if os(tvOS)
        HStack(alignment: .center, spacing: 96) {
            VStack(alignment: .leading, spacing: 40) {
                header
                if quickConnectAvailable { quickConnectCard }
            }
            .frame(width: 720, alignment: .leading)
            VStack(spacing: 28) { account; failureLabel }
                .frame(width: 640)
                .focusSection()
        }
        .padding(.horizontal, KinoTheme.contentPadding)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        #else
        ScrollView {
            VStack(spacing: 32) {
                header
                account
                if loaded && quickConnectAvailable { quickConnectCard }
                failureLabel
            }
            .padding(.horizontal, KinoTheme.contentPadding)
            .padding(.vertical, 24)
            .frame(maxWidth: 620)
            .frame(maxWidth: .infinity)
        }
        #endif
    }

    @ViewBuilder private var account: some View {
        if !loaded {
            ProgressView().controlSize(.large).padding(.top, 24).accessibilityLabel("Loading sign-in options")
        } else {
            Group {
                if let selected { passwordCard(selected) }
                else if !users.isEmpty && !manual { picker }
                else { manualCard }
            }
            .transition(reduceMotion ? .opacity : .opacity.combined(with: .scale(scale: 0.97)))
        }
    }

    @ViewBuilder private var failureLabel: some View {
        if let failure {
            Label(failure, systemImage: "exclamationmark.triangle.fill")
                .font(.callout).foregroundStyle(.red)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    private var header: some View {
        VStack(alignment: tv ? .leading : .center, spacing: 14) {
            ServerMonogram(brand: .source(server.kind), size: tv ? 96 : 72)
            Text(server.name).font(.system(.title, design: .rounded).bold()).foregroundStyle(KinoTheme.text)
                .multilineTextAlignment(tv ? .leading : .center).accessibilityAddTraits(.isHeader)
            Text("\(server.kind.title) \(server.version) · \(server.address.host ?? "")")
                .font(.subheadline).foregroundStyle(KinoTheme.muted)
        }
        .frame(maxWidth: .infinity, alignment: tv ? .leading : .center)
    }

    private var tv: Bool {
        #if os(tvOS)
        true
        #else
        false
        #endif
    }

    private var picker: some View {
        VStack(spacing: 20) {
            Text("Who’s watching?").font(.title2.bold()).foregroundStyle(KinoTheme.text).accessibilityAddTraits(.isHeader)
            LazyVGrid(columns: Array(repeating: GridItem(.fixed(avatarSize + 24), spacing: 20), count: min(users.count, columns)), spacing: 24) {
                ForEach(users) { person in
                    Button { choose(person) } label: {
                        VStack(spacing: 10) {
                            UserAvatar(name: person.name, image: avatars[person.id], size: avatarSize)
                            Text(person.name).font(.callout.weight(.semibold)).foregroundStyle(KinoTheme.text).lineLimit(1)
                        }
                    }
                    .serverCardButtonStyle()
                    .disabled(working)
                }
            }
            Button("Sign in as someone else") { manual = true; focus = .user }
                .font(.subheadline.weight(.semibold)).tint(KinoTheme.signal)
        }
    }

    private func passwordCard(_ person: SourcePublicUser) -> some View {
        VStack(spacing: 18) {
            UserAvatar(name: person.name, image: avatars[person.id], size: 80)
            Text("Welcome back, \(person.name)").font(.title3.bold()).foregroundStyle(KinoTheme.text)
            secure
            signInButton { submit(person.name) }
            Button("Not \(person.name)?") { selected = nil; password = ""; failure = nil }
                .font(.subheadline.weight(.semibold)).tint(KinoTheme.signal)
        }
        .card()
        .onAppear { focus = .password }
    }

    private var manualCard: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text(quickConnectAvailable ? "Sign in with a password" : "Sign in").font(.headline).foregroundStyle(KinoTheme.text)
            TextField("User name", text: $user, prompt: Text("User name").foregroundStyle(KinoTheme.muted))
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .textContentType(.username)
                .submitLabel(.next)
                .focused($focus, equals: .user)
                .onSubmit { focus = .password }
                .fieldSurface()
            secure
            signInButton { submit(user) }
            if !users.isEmpty {
                Button("Choose a profile instead") { manual = false }.font(.subheadline.weight(.semibold)).tint(KinoTheme.signal)
            }
        }
        .card()
    }

    private var secure: some View {
        SecureField("Password", text: $password, prompt: Text("Password").foregroundStyle(KinoTheme.muted))
            .textContentType(.password)
            .submitLabel(.go)
            .focused($focus, equals: .password)
            .onSubmit { submit(selected?.name ?? user) }
            .fieldSurface()
    }

    private func signInButton(_ action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text("Sign in").font(.headline).foregroundStyle(KinoTheme.actionInk)
                .opacity(working ? 0 : 1)
                .overlay { if working { ProgressView().tint(KinoTheme.signalInk) } }
                .frame(maxWidth: .infinity, minHeight: 40)
        }
        .primaryCapsule()
        .disabled(working || (selected == nil && user.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty))
        .accessibilityLabel(working ? "Signing in" : "Sign in")
    }

    private var quickConnectCard: some View {
        VStack(alignment: .leading, spacing: 12) {
            Label("Quick Connect", systemImage: "bolt.fill").font(.headline).foregroundStyle(KinoTheme.signal)
            if let quickConnect {
                HStack(spacing: 28) {
                    VStack(spacing: 12) {
                        Text(quickConnect.code)
                            .font(.system(size: tv ? 88 : 46, weight: .bold, design: .rounded).monospacedDigit())
                            .tracking(tv ? 14 : 10)
                            .foregroundStyle(KinoTheme.text)
                            .minimumScaleFactor(0.6)
                            .lineLimit(1)
                            .contentTransition(.numericText())
                            .accessibilityLabel("Quick Connect code \(quickConnect.code.map(String.init).joined(separator: " "))")
                        HStack(spacing: 8) {
                            ScanPulse(active: true)
                            Text("Waiting for approval…").font(.callout).foregroundStyle(KinoTheme.muted)
                        }
                    }
                    .frame(maxWidth: .infinity)
                    if showsQR, let url = try? QuickConnectLink(serverID: server.serverID, code: quickConnect.code).url {
                        QRCodeView(value: url.absoluteString)
                            .frame(width: tv ? 200 : 150, height: tv ? 200 : 150)
                            .accessibilityLabel("Scan with your iPhone to approve this sign-in")
                    }
                }
            } else {
                ProgressView().frame(maxWidth: .infinity, minHeight: 60)
            }
            Text(showsQR ? "Scan the code with your iPhone camera to approve in Kinosail. Or, in Jellyfin, open your profile, choose Quick Connect and enter the number."
                 : "In Jellyfin on your phone or computer, open your profile, choose Quick Connect and enter this code. No password needed.")
                .font(.footnote).foregroundStyle(KinoTheme.muted)
        }
        .card()
    }

    private var showsQR: Bool {
        #if os(tvOS)
        true
        #else
        UIDevice.current.userInterfaceIdiom == .pad
        #endif
    }

    private var columns: Int {
        #if os(tvOS)
        4
        #else
        3
        #endif
    }

    private var avatarSize: CGFloat {
        #if os(tvOS)
        150
        #else
        84
        #endif
    }

    private func load() async {
        async let enabled = SourceConnect.quickConnectEnabled(server)
        async let people = SourceConnect.publicUsers(server)
        let (quick, found) = await (enabled, people)
        quickConnectAvailable = quick
        users = found
        if found.count == 1, let only = found.first, only.hasPassword { selected = only }
        loaded = true
        for person in found where person.hasImage {
            if let data = await SourceConnect.avatar(server, user: person), let image = UIImage(data: data) { avatars[person.id] = image }
        }
    }

    private func choose(_ person: SourcePublicUser) {
        failure = nil
        if person.hasPassword { selected = person; password = "" }
        else { submit(person.name) }
    }

    private func runQuickConnect() async {
        guard quickConnectAvailable else { return }
        do {
            let pending = try await SourceConnect.startQuickConnect(server)
            quickConnect = pending
            let account = try await SourceConnect.awaitQuickConnect(server, pending, until: Date().addingTimeInterval(270))
            finish(account)
        } catch is CancellationError {
        } catch ClientError.invalidInput {
            codeRevision += 1
        } catch {
            quickConnect = nil
            quickConnectAvailable = false
        }
    }

    private func submit(_ name: String) {
        guard !working else { return }
        working = true
        failure = nil
        Task {
            defer { working = false }
            do { finish(try await SourceConnect.signIn(server, user: name, password: password)) }
            catch { failure = AppSession.message(error) }
        }
    }

    private func finish(_ account: SourceAccount) {
        password = ""
        signedIn(account)
    }
}
