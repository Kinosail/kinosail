import SwiftUI

struct ApprovalScreen: View {
    var initialCode = ""
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var code = ""
    @State private var approval: DeviceApproval?
    @State private var approvalIdentity: UUID?
    @State private var busy = false
    @State private var error: String?
    @State private var approved = false

    var body: some View {
        Form {
            if approved {
                Section {
                    Label("Device connected", systemImage: "checkmark.circle")
                    Button("Done") { session.dismissApproval(); dismiss() }
                }
            } else if let approval {
                Section("Review device") {
                    LabeledContent("Device", value: approval.device.isEmpty ? "Kinosail device" : approval.device)
                    LabeledContent("Code", value: approval.code)
                    LabeledContent("Viewer Profile", value: session.viewer?.name ?? "")
                    Text("Approve only if this code matches the device you want to connect.")
                    Button(busy ? "Approving…" : "Approve device") { approve(approval) }
                        .disabled(busy || approval.expires <= Date())
                    Button("Not now") { session.dismissApproval(); dismiss() }
                        .disabled(busy)
                }
            } else {
                Section("Connect a TV") {
                    TextField("Six-digit code", text: $code)
                        #if os(iOS)
                        .keyboardType(.numberPad)
                        #endif
                        .onChange(of: code) { _, _ in error = nil }
                    Button(busy ? "Checking…" : "Review device") { Task { await review() } }
                        .disabled(busy || code.isEmpty)
                }
            }
            if let error { Section { Text(error).foregroundStyle(.red) } }
        }
        .tvOSConfigurationLayout(title: "Connect a TV", symbol: "tv")
        .configurationNavigationTitle("Connect a TV")
        .onChange(of: session.client?.identity) { _, _ in
            approval = nil
            approvalIdentity = nil
            approved = false
        }
        .task {
            if !initialCode.isEmpty, code.isEmpty { code = initialCode; await review() }
        }
    }

    private func review() async {
        guard !busy, let client = session.client else { return }
        busy = true
        error = nil
        defer { busy = false }
        do {
            let pending = try await client.previewApproval(code: code)
            guard session.client?.identity == client.identity else { return }
            approvalIdentity = client.identity
            approval = pending
        }
        catch { self.error = AppSession.message(error) }
    }

    private func approve(_ pending: DeviceApproval) {
        guard !busy, pending.expires > Date(), let client = session.client, approvalIdentity == client.identity else { return }
        busy = true
        error = nil
        Task {
            defer { busy = false }
            do {
                guard session.client?.identity == client.identity, approvalIdentity == client.identity else { return }
                try await client.approveDevice(code: pending.code)
                guard session.client?.identity == client.identity else { return }
                approved = true
            } catch { self.error = AppSession.message(error) }
        }
    }
}
