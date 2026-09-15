import SwiftUI
import CoreImage.CIFilterBuiltins

struct SetupScreen: View {
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var address = ""
    @State private var discovery = BonjourDiscovery()
    @State private var scanError: String?
    @State private var scanRevision = 0

    var body: some View {
        NavigationStack {
            Group {
                if let code = session.pairingCode, let server = session.pairingAddress {
                    PairingCodeScreen(code: code, server: server)
                } else {
                    Form {
                        Section {
                            TextField("https://your-server", text: $address)
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                                #if os(iOS)
                                .keyboardType(.URL)
                                .textContentType(.URL)
                                #endif
                                .accessibilityLabel("Server address")
                                .onSubmit { connect() }
                            Button(session.connecting ? "Connecting…" : "Connect") { connect() }
                                .disabled(address.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || session.connecting)
                        } header: { Text("Server address") } footer: {
                            Text("Your media stays on your Server. Kinosail connects directly to it.")
                        }
                        Section("Nearby") {
                            ForEach(discovery.servers) { server in
                                Button {
                                    address = server.address.url.absoluteString
                                    connect()
                                } label: {
                                    VStack(alignment: .leading, spacing: 4) {
                                        Text(server.name)
                                        Text(server.address.url.absoluteString).font(.caption).foregroundStyle(.secondary)
                                    }
                                }
                                .disabled(session.connecting)
                            }
                            if discovery.servers.isEmpty {
                                Text(discovery.scanning ? "Looking for your Server…" : "No nearby Servers found.")
                                    .foregroundStyle(.secondary)
                            }
                            Button(discovery.scanning ? "Searching…" : "Search again") { scanRevision += 1 }
                                .disabled(discovery.scanning)
                            if let scanError { Text(scanError).foregroundStyle(.secondary) }
                        }
                    }
                    .task(id: scanRevision) {
                        scanError = nil
                        do { try await discovery.scan() }
                        catch is CancellationError {}
                        catch { scanError = AppSession.message(error) }
                    }
                }
            }
            #if os(tvOS)
            .navigationTitle("")
            .toolbar {
                ToolbarItem(placement: .principal) {
                    Text("Connect to your library")
                        .font(.title2.bold())
                        .foregroundStyle(KinoTheme.text)
                        .accessibilityAddTraits(.isHeader)
                }
            }
            #else
            .navigationTitle("Connect to your library")
            #endif
            .toolbar {
                if session.client != nil {
                    ToolbarItem(placement: .cancellationAction) {
                        Button("Cancel") { Task { await session.cancelPairing(); dismiss() } }
                    }
                }
            }
            .onDisappear { discovery.stop() }
        }
    }

    private func connect() {
        guard !session.connecting else { return }
        discovery.stop()
        Task { await session.connect(address: address) }
    }
}

struct PairingCodeScreen: View {
    let code: String
    let server: ServerAddress
    @Environment(AppSession.self) private var session

    var body: some View {
        ScrollView {
            VStack(spacing: 24) {
                Text("Approve this device").font(.title.bold())
                if let url = try? ApprovalLink.url(server: server, code: code) {
                    QRCodeView(value: url.absoluteString)
                        .frame(width: 224, height: 224)
                        .accessibilityLabel("Scan with your phone to approve this device")
                }
                Text(code).font(.system(size: 44, weight: .semibold, design: .monospaced))
                    .accessibilityLabel("Approval code \(code.map(String.init).joined(separator: " "))")
                Text("Scan the QR code with your phone, or enter the six-digit code in Settings → Connect a TV on a signed-in Player.")
                    .multilineTextAlignment(.center).frame(maxWidth: 540)
                #if os(iOS)
                if let url = try? server.mediaURL("/quick-connect?code=\(code)") {
                    Link("Approve in browser", destination: url)
                }
                #endif
                Text(server.url.absoluteString).font(.caption).foregroundStyle(.secondary)
                Text("Waiting for approval…").font(.callout).foregroundStyle(.secondary)
                Button("Cancel") { Task { await session.cancelPairing() } }
            }
            .padding(KinoTheme.contentPadding)
            .frame(maxWidth: .infinity)
        }
    }
}

private struct QRCodeView: View {
    let value: String
    var body: some View {
        Group {
            if let image = qrImage {
                Image(uiImage: image).interpolation(.none).resizable().scaledToFit()
            } else {
                Image(systemName: "qrcode").resizable().scaledToFit().padding(32)
                    .accessibilityLabel("Use the six-digit code below")
            }
        }
        .padding(16).background(.white).clipShape(.rect(cornerRadius: 12))
    }

    private var qrImage: UIImage? {
        let filter = CIFilter.qrCodeGenerator()
        filter.message = Data(value.utf8)
        filter.correctionLevel = "M"
        guard let output = filter.outputImage?.transformed(by: CGAffineTransform(scaleX: 6, y: 6)),
              let image = CIContext().createCGImage(output, from: output.extent) else { return nil }
        return UIImage(cgImage: image)
    }
}
