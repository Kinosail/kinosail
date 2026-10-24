import Foundation

extension AppSession {
    var usesDeviceLibrary: Bool { DeviceLibraryHost.isDeviceLibrary(viewer) }

    func connectLibrary(_ account: SourceAccount) async throws {
        try await DeviceLibraryHost.shared.add(account)
        if usesDeviceLibrary {
            await DeviceLibraryHost.shared.refresh()
            resourceSnapshots.clear()
            showsSetup = false
            contentRevision = UUID()
            return
        }
        await cancelPairing()
        let attempt = generation
        let (server, token) = try await DeviceLibraryHost.shared.start()
        let candidate = try ServerClient(server: server, token: token)
        do {
            let profile = try await candidate.viewer()
            try await adopt(candidate, server: server, token: token, profile: profile, attempt: attempt)
        } catch {
            await candidate.close()
            throw error
        }
    }

    func removeLibrary(_ id: String) async throws {
        try await DeviceLibraryHost.shared.remove(id)
        if DeviceLibraryHost.shared.sources.isEmpty { await disconnect(); return }
        resourceSnapshots.clear()
        contentRevision = UUID()
    }

    func resumeDeviceLibrary() async {
        guard usesDeviceLibrary, let current = client else { return }
        do {
            let (server, _) = try await DeviceLibraryHost.shared.start()
            if server != current.server {
                restoreAfterRestart()
            }
        } catch { notice = Self.message(error) }
    }

    private func restoreAfterRestart() {
        Task { await restart() }
    }

    private func restart() async {
        await cancelPairing()
        let attempt = generation
        do {
            let (server, token) = try await DeviceLibraryHost.shared.start()
            let candidate = try ServerClient(server: server, token: token)
            let profile = try await candidate.viewer()
            try await adopt(candidate, server: server, token: token, profile: profile, attempt: attempt)
        } catch { notice = Self.message(error) }
    }
}
