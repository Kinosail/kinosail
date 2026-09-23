import Foundation
import CryptoKit
import Observation

@MainActor @Observable
final class AppSession {
    private(set) var client: ServerClient?
    private(set) var viewer: Viewer?
    private(set) var restoring = true
    private(set) var connecting = false
    let connection = ConnectionStatus()
    private(set) var pairingCode: String?
    private(set) var pairingAddress: ServerAddress?
    var showsSetup = false
    var showsVideoPlayer = false
    var notice: String?
    private(set) var pendingApprovalCode: String?
    var reading = false
    var pendingMediaLink: MediaLink?
    var contentRevision = UUID()
    var supporterRevision = UUID()
    let player = PlaybackCoordinator()
    let casting = CastCoordinator()
    let artwork = ArtworkLoader()
    private(set) var progress: ProgressSyncStore?
    #if os(iOS)
    let downloads = OfflineDownloadManager()
    #endif

    private let keychain = SessionKeychain()
    private var restoreTask: Task<Void, Never>?
    private var generation = UUID()
    private var pairingTask: Task<Void, Never>?
    private var pairingClient: ServerClient?
    private var challenge: ConnectChallenge?

    init() {
        #if os(tvOS)
        TopShelfPreferences.migrateToDefaultOn()
        #endif
        #if os(iOS)
        player.onCompleted = { [weak self] item, nextID in
            guard let self, let client = self.client else { return }
            Task { await self.downloads.completed(item, nextID: nextID, client: client) }
        }
        #endif
    }

    var profileKey: String? {
        guard let client, let viewer else { return nil }
        let raw = "\(client.server.url.absoluteString)\n\(viewer.serverID)\n\(viewer.id)"
        return SHA256.hash(data: Data(raw.utf8)).map { String(format: "%02x", $0) }.joined()
    }

    func restore() async {
        if restoreTask == nil { restoreTask = Task { await restoreSession() } }
        await restoreTask?.value
    }

    private func restoreSession() async {
        let attempt = generation
        var restoredClient: UUID?
        defer { restoring = false }
        do {
            guard let saved = try await keychain.restore() else { return }
            let candidate = try ServerClient(server: saved.server, token: saved.token, viewer: saved.viewer)
            guard generation == attempt else { await candidate.close(); return }
            // Keychain identifies the local cache immediately. Server permissions
            // are revalidated while saved browsing content is already available.
            client = candidate
            restoredClient = candidate.identity
            viewer = saved.viewer
            configureProgress()
            restoring = false
            #if os(iOS)
            // Restore local media before waiting for a possibly unreachable Server.
            await downloads.restore(viewer: saved.viewer, server: saved.server, client: candidate)
            guard client?.identity == candidate.identity else { await candidate.close(); return }
            #endif
            let profile: Viewer
            do { profile = try await candidate.viewer() }
            catch let error as ClientError where error.permitsOfflineRestore {
                profile = saved.viewer
            }
            catch {
                let revoked = error as? ClientError == .http(401) || error as? ClientError == .http(403)
                await candidate.close(purgeCache: revoked)
                if client?.identity == candidate.identity {
                    player.stop()
                    clearSystemContent()
                    client = nil; viewer = nil; progress = nil
                    #if os(iOS)
                    await downloads.lock()
                    #endif
                    await artwork.clear()
                    if revoked { try await keychain.discard(saved) }
                    notice = Self.message(error)
                }
                throw error
            }
            guard client?.identity == candidate.identity else { await candidate.close(); return }
            let updated = try SavedSession(server: saved.server, token: saved.token, viewer: profile)
            guard try await keychain.refresh(updated) else { return }
            guard client?.identity == candidate.identity else { try await keychain.discard(updated); await candidate.close(); return }
            viewer = profile
            #if os(iOS)
            if profile.downloads != saved.viewer.downloads {
                await downloads.restore(viewer: profile, server: saved.server, client: candidate)
            }
            #endif
        } catch { if generation == attempt || client?.identity == restoredClient { notice = Self.message(error) } }
    }

    func connect(address: String) async {
        await cancelPairing()
        let attempt = UUID()
        generation = attempt
        do {
            let address = try ServerAddress(address)
            let candidate = try ServerClient(server: address)
            connecting = true
            pairingAddress = address
            pairingClient = candidate
            notice = nil
            pairingTask = Task { await pair(candidate: candidate, attempt: attempt) }
        } catch { notice = Self.message(error) }
    }

    private func pair(candidate: ServerClient, attempt: UUID) async {
        var pending: ConnectChallenge?
        do {
            let created = try await candidate.startQuickConnect(device: Self.deviceName)
            pending = created
            try Task.checkCancellation()
            guard generation == attempt else { throw CancellationError() }
            challenge = created
            pairingCode = created.code
            let deadline = Date().addingTimeInterval(300)
            while Date() < deadline {
                try await Task.sleep(for: .seconds(2))
                guard generation == attempt else { throw CancellationError() }
                if let token = try await candidate.pollQuickConnect(secret: created.secret) {
                    let authenticated = try ServerClient(server: candidate.server, token: token)
                    do {
                        let profile = try await authenticated.viewer()
                        guard generation == attempt, !Task.isCancelled else { throw CancellationError() }
                        let saved = try SavedSession(server: candidate.server, token: token, viewer: profile)
                        try await keychain.save(saved)
                        guard generation == attempt, !Task.isCancelled else {
                            try await keychain.discard(saved)
                            throw CancellationError()
                        }
                        player.stop()
                        await casting.clear()
                        await artwork.clear()
                        guard generation == attempt, !Task.isCancelled else { try await keychain.discard(saved); throw CancellationError() }
                        let previous = client
                        clearSystemContent()
                        client = authenticated
                        viewer = profile
                        configureProgress()
                        showsSetup = false
                        contentRevision = UUID()
                        #if os(iOS)
                        await downloads.restore(viewer: profile, server: candidate.server, client: authenticated)
                        #endif
                        if let previous { await previous.close() }
                    } catch {
                        // This token was issued by the abandoned attempt; never leave it active.
                        try? await authenticated.signOut()
                        await authenticated.close()
                        throw error
                    }
                    finishPairing(attempt: attempt)
                    await candidate.close()
                    return
                }
            }
            throw ClientError.invalidInput("That code expired. Connect again to get a new code.")
        } catch {
            if let pending { try? await candidate.cancelQuickConnect(secret: pending.secret) }
            await candidate.close()
            if generation == attempt {
                finishPairing(attempt: attempt)
                if !(error is CancellationError) { notice = Self.message(error) }
            }
        }
    }

    private func finishPairing(attempt: UUID) {
        guard generation == attempt else { return }
        connecting = false
        pairingCode = nil
        pairingAddress = nil
        challenge = nil
        pairingClient = nil
        pairingTask = nil
    }

    func cancelPairing() async {
        generation = UUID()
        pairingTask?.cancel()
        pairingTask = nil
        let previous = pairingClient
        let pending = challenge
        pairingClient = nil
        challenge = nil
        pairingCode = nil
        pairingAddress = nil
        connecting = false
        if let previous {
            if let pending { try? await previous.cancelQuickConnect(secret: pending.secret) }
            await previous.close()
        }
    }

    func disconnect() async {
        clearSystemContent()
        await cancelPairing()
        let attempt = generation
        player.stop()
        await casting.clear()
        guard generation == attempt else { return }
        do { try await keychain.clear() }
        catch { notice = Self.message(error); return }
        guard generation == attempt else { return }
        await artwork.clear()
        guard generation == attempt else { return }
        let previous = client
        client = nil
        viewer = nil
        progress = nil
        pendingApprovalCode = nil
        pendingMediaLink = nil
        showsSetup = false
        contentRevision = UUID()
        #if os(iOS)
        await downloads.lock()
        #endif
        if let previous {
            do { try await previous.signOut() }
            catch { notice = "Signed out on this device. The Server could not be reached to revoke its session." }
            await previous.close()
        }
    }

    private func configureProgress() {
        guard let scope = profileKey else { progress = nil; return }
        do { progress = try ProgressSyncStore(scope: scope) }
        catch { progress = nil; notice = Self.message(error) }
    }

    private func clearSystemContent() {
        pendingMediaLink = nil
        #if os(tvOS)
        TopShelfPublishing.clear()
        #endif
    }

    func handleIncomingURL(_ url: URL) {
        do {
            if url.host == "media" {
                try openMediaLink(MediaLink(url: url))
                return
            }
            guard let client else { throw ClientError.invalidInput("Connect to your Server before approving a TV.") }
            pendingApprovalCode = try ApprovalLink.code(url, server: client.server)
        } catch { notice = Self.message(error) }
    }

    func openMediaLink(_ link: MediaLink) throws {
        guard link.scope == profileKey else { throw ClientError.invalidInput("Open this title using the Server and Viewer Profile it belongs to.") }
        pendingMediaLink = link
    }

    func dismissApproval() {
        pendingApprovalCode = nil
    }

    static func message(_ error: any Error) -> String {
        (error as? ClientError)?.localizedDescription ?? "The operation could not finish. Please try again."
    }

    private static var deviceName: String {
        #if os(tvOS)
        "Kinosail Apple TV"
        #else
        "Kinosail iPhone or iPad"
        #endif
    }
}
