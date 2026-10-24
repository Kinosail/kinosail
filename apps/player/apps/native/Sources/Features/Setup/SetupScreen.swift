import SwiftUI

enum SetupStep: Hashable {
    case signIn(SourceServerInfo)
    case ready(SourceAccount)
}

struct SetupScreen: View {
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var address = ""
    @State private var discovery = BonjourDiscovery()
    @State private var sources: [SourceServerInfo] = []
    @State private var searchingSources = false
    @State private var scanError: String?
    @State private var scanRevision = 0
    @State private var probing = false
    @State private var path: [SetupStep] = []
    @State private var added: [SourceAccount] = []
    @State private var starting = false
    @State private var shared: [SourceAccount] = []

    private var adding: Bool { session.client != nil }
    private var scanning: Bool { discovery.scanning || searchingSources }
    private var found: Int { discovery.servers.count + sources.count }

    var body: some View {
        NavigationStack(path: $path) {
            Group {
                if let code = session.pairingCode, let server = session.pairingAddress {
                    PairingCodeScreen(code: code, server: server)
                } else {
                    welcome
                }
            }
            .tvOSConfigurationLayout(title: adding ? "Add a server" : "Connect your library", symbol: "server.rack")
            #if os(tvOS)
            .navigationTitle("")
            #else
            .navigationTitle(adding ? "Add a server" : "")
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                if adding {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Cancel") { Task { await session.cancelPairing(); dismiss() } }
                    }
                }
            }
            .navigationDestination(for: SetupStep.self) { step in
                switch step {
                case .signIn(let server):
                    SourceSignInScreen(server: server) { path.append(.ready($0)) }
                case .ready(let account):
                    SourceReadyScreen(account: account, earlier: added, addAnother: { try await keep(account) },
                                      start: { try await session.connectLibrary(account) })
                }
            }
        }
        .onDisappear { discovery.stop() }
    }

    private var welcome: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 32) {
                #if os(iOS)
                SetupBrandHeader(adding: adding)
                #endif
                SharedServersSection(shared: shared, excluded: Set((DeviceLibraryHost.shared.sources + added).map(\.id))) { path.append(.ready($0)) }
                nearby
                manual
                VStack(alignment: .leading, spacing: 12) {
                    Text("Works with Kinosail, Jellyfin and Emby. Your media stays on your server. Servers you sign in to sync to your other Apple devices with iCloud Keychain.")
                        .font(.footnote).foregroundStyle(KinoTheme.muted)
                    PrivacyPolicyLink().font(.footnote)
                }
            }
            .padding(.horizontal, KinoTheme.contentPadding)
            .padding(.vertical, 24)
            .frame(maxWidth: 620)
            .frame(maxWidth: .infinity)
        }
        #if os(iOS)
        .scrollDismissesKeyboard(.interactively)
        .cinemaBackground()
        #endif
        .safeAreaInset(edge: .bottom) { if !added.isEmpty { startBar } }
        .task(id: scanRevision) { await scan() }
        .task { shared = await SourceSharedStore.shared.load() }
    }

    private var nearby: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(spacing: 10) {
                ScanPulse(active: scanning)
                Text("On your network").font(.title3.bold()).foregroundStyle(KinoTheme.text).accessibilityAddTraits(.isHeader)
                Spacer()
                if !scanning {
                    Button("Search again") { scanRevision += 1 }
                        .font(.subheadline.weight(.semibold)).tint(KinoTheme.signal)
                }
            }
            VStack(spacing: 12) {
                ForEach(discovery.servers) { server in
                    Button { connectKinosail(server.address.url.absoluteString) } label: {
                        ServerCard(brand: .kinosail, name: server.name, detail: "Kinosail · \(server.address.url.host ?? "")", badge: "Best experience")
                    }
                    .serverCardButtonStyle()
                    .disabled(session.connecting)
                    .cardAppearance(reduceMotion: reduceMotion)
                }
                ForEach(sources) { server in
                    let connected = isConnected(server)
                    Button { path.append(.signIn(server)) } label: {
                        ServerCard(brand: .source(server.kind), name: server.name, detail: "\(server.kind.title) · \(server.address.host ?? "")", connected: connected)
                    }
                    .serverCardButtonStyle()
                    .disabled(connected || session.connecting)
                    .cardAppearance(reduceMotion: reduceMotion)
                }
                if found == 0 {
                    if scanning {
                        ServerCardPlaceholder()
                        ServerCardPlaceholder().opacity(0.55)
                    } else {
                        Text("No servers found nearby. Check that this device is on the same Wi‑Fi as your server, or enter its address below.")
                            .font(.callout).foregroundStyle(KinoTheme.muted)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
            }
            .animation(reduceMotion ? .easeInOut(duration: 0.2) : .spring(duration: 0.5, bounce: 0.22), value: found)
            .animation(.easeInOut(duration: 0.25), value: scanning)
            if let scanError { Text(scanError).font(.footnote).foregroundStyle(KinoTheme.muted) }
        }
        .accessibilityLabel(scanning ? "Searching your network" : "Servers on your network")
    }

    private var manual: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Or enter an address").font(.headline).foregroundStyle(KinoTheme.text).accessibilityAddTraits(.isHeader)
            #if os(iOS)
            HStack(spacing: 8) { addressField; connectButton }
                .padding(.leading, 16)
                .padding(.trailing, 6)
                .frame(minHeight: 56)
                .background(KinoTheme.surface.opacity(0.86), in: .rect(cornerRadius: 18, style: .continuous))
                .overlay(RoundedRectangle(cornerRadius: 18, style: .continuous).strokeBorder(.white.opacity(0.06), lineWidth: 1))
            #else
            addressField
            connectButton
            #endif
        }
    }

    private var addressField: some View {
        TextField("Server address", text: $address, prompt: Text("192.168.1.20 or https://media.example.com").foregroundStyle(KinoTheme.muted))
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            #if os(iOS)
            .keyboardType(.URL)
            .textContentType(.URL)
            .submitLabel(.go)
            #endif
            .accessibilityLabel("Server address")
            .onSubmit { connect() }
    }

    private var connectButton: some View {
        let busy = probing || session.connecting
        let empty = address.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        return Button { connect() } label: {
            #if os(iOS)
            ZStack {
                Circle().fill(empty ? KinoTheme.raised : KinoTheme.signal)
                if busy { ProgressView().tint(KinoTheme.signalInk) }
                else { Image(systemName: "arrow.right").font(.headline.weight(.bold)).foregroundStyle(empty ? KinoTheme.muted : KinoTheme.signalInk) }
            }
            .frame(width: 44, height: 44)
            #else
            Text(busy ? "Connecting…" : "Connect").foregroundStyle(KinoTheme.actionInk).frame(maxWidth: .infinity)
            #endif
        }
        .connectButtonStyle()
        .animation(.easeInOut(duration: 0.2), value: empty)
        .disabled(empty || busy)
        .accessibilityLabel(busy ? "Connecting" : "Connect")
    }

    private var startBar: some View {
        Button {
            guard let last = added.last, !starting else { return }
            starting = true
            Task { defer { starting = false }; try? await session.connectLibrary(last) }
        } label: {
            Label(added.count == 1 ? "Start watching" : "Start watching · \(added.count) servers", systemImage: "play.fill")
                .font(.headline)
                .foregroundStyle(KinoTheme.actionInk)
                .frame(maxWidth: .infinity, minHeight: 44)
        }
        .primaryCapsule()
        .disabled(starting)
        .padding(.horizontal, KinoTheme.contentPadding)
        .padding(.vertical, 12)
        .frame(maxWidth: 620)
        .frame(maxWidth: .infinity)
        .background(.ultraThinMaterial)
    }

    private func scan() async {
        scanError = nil
        searchingSources = true
        async let others: Void = SourceDiscovery.scan { found in
            guard !sources.contains(where: { $0.id == found.id }) else { return }
            sources.append(found)
        }
        do { try await discovery.scan() }
        catch is CancellationError {}
        catch { scanError = AppSession.message(error) }
        await others
        searchingSources = false
    }

    private func isConnected(_ server: SourceServerInfo) -> Bool {
        (DeviceLibraryHost.shared.sources + added).contains { $0.kind == server.kind && $0.serverID == server.serverID }
            && (adding ? session.usesDeviceLibrary || !added.isEmpty : !added.isEmpty)
    }

    private func keep(_ account: SourceAccount) async throws {
        try await DeviceLibraryHost.shared.add(account)
        added.removeAll { $0.id == account.id }
        added.append(account)
        path = []
    }

    private func connect() {
        guard !session.connecting, !probing else { return }
        probing = true
        Task {
            defer { probing = false }
            if let url = try? SourceAddress.parse(address, defaultPort: 8096), let server = try? await SourceConnect.probe(url) {
                path.append(.signIn(server))
                return
            }
            connectKinosail(address)
        }
    }

    private func connectKinosail(_ value: String) {
        guard !session.connecting else { return }
        discovery.stop()
        Task { await session.connect(address: value) }
    }
}

private extension View {
    @ViewBuilder func connectButtonStyle() -> some View {
        #if os(iOS)
        buttonStyle(.plain)
        #else
        primaryCapsule()
        #endif
    }
}

#if os(iOS)
private struct SetupBrandHeader: View {
    let adding: Bool
    @ScaledMetric(relativeTo: .largeTitle) private var titleSize = 34.0

    var body: some View {
        VStack(spacing: 14) {
            Image("LaunchMark")
                .resizable().scaledToFit()
                .frame(width: 52, height: 67)
                .shadow(color: KinoTheme.signal.opacity(0.35), radius: 18)
                .accessibilityHidden(true)
            Text(adding ? "Add a server" : "Kinosail Player")
                .font(.system(size: titleSize, weight: .bold, design: .rounded))
                .tracking(-0.4)
                .accessibilityAddTraits(.isHeader)
            Text(adding ? "Titles from every server appear together in one library." : "All your movies and shows, from every server, in one place.")
                .font(.subheadline).foregroundStyle(KinoTheme.muted)
                .multilineTextAlignment(.center)
        }
        .foregroundStyle(KinoTheme.text)
        .frame(maxWidth: .infinity)
        .padding(.top, adding ? 0 : 24)
    }
}
#endif
