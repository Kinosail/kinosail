import Foundation
import Observation

@MainActor @Observable
final class CastCoordinator {
    private(set) var session: CastSession?
    private(set) var status: CastStatus?
    private(set) var itemID: String?
    private(set) var busy = false
    private(set) var message: String?
    private var client: ServerClient?
    private var writer: ProgressWriter?
    private var generation = UUID()
    private var lastSaved = Date.distantPast

    func start(item: MediaItem, device: CastDevice, position: Double, client: ServerClient, store: ProgressSyncStore) async throws {
        guard !busy, session == nil, item.kind == .video || item.isAudio, device.receiverProtocol == .dlna else { throw ClientError.invalidInput("Stop the active TV session before starting another.") }
        try Input.position(position)
        let writer = try ProgressWriter(itemID: item.id, expected: item.progress, client: client, store: store)
        busy = true
        let attempt = generation
        defer { if generation == attempt { busy = false } }
        let result = try await client.startCast(itemID: item.id, request: CastStart(receiverProtocol: .dlna, deviceID: device.id, position: position, playbackToken: nil))
        guard generation == attempt, !Task.isCancelled else { try? await client.endCast(id: result.id); throw CancellationError() }
        self.session = result; self.client = client; self.itemID = item.id; self.writer = writer
        self.status = nil; message = nil; lastSaved = .distantPast
    }

    func command(_ command: CastCommand) async throws {
        guard !busy, let session, let client else { throw ClientError.invalidInput("Connect to a TV first.") }
        if case .seek(let seconds) = command { guard session.duration > 0, seconds <= session.duration else { throw ClientError.invalidInput("Choose a position within this title.") } }
        busy = true
        let attempt = generation
        defer { if generation == attempt { busy = false } }
        try await client.castCommand(id: session.id, command: command)
        guard generation == attempt else { return }
        if case .stop = command { await clear(revoke: false); return }
        status = nil
        message = nil
    }

    func monitor() async {
        while !Task.isCancelled {
            if !busy, let session, let client {
                let attempt = generation
                do {
                    let next = try await client.castStatus(id: session.id)
                    guard generation == attempt, !Task.isCancelled else { return }
                    status = next; message = nil
                    if Date().timeIntervalSince(lastSaved) >= 15, let writer {
                        lastSaved = Date()
                        let synced = try await writer.update(seconds: next.position, watched: next.state == .stopped && next.duration > 0 && next.position >= next.duration - 2)
                        if generation == attempt, !synced { message = "Your TV position is saved on this device. Open Progress sync to finish syncing." }
                    }
                } catch is CancellationError { return }
                catch { if generation == attempt { message = AppSession.message(error) } }
            }
            do { try await Task.sleep(for: .seconds(4)) } catch { return }
        }
    }

    func disconnect() async throws {
        guard !busy, let session, let client else { return }
        busy = true
        let attempt = generation
        defer { if generation == attempt { busy = false } }
        try await client.endCast(id: session.id)
        guard generation == attempt else { return }
        await clear(revoke: false)
    }

    func clear(revoke: Bool = true) async {
        let previous = session, previousClient = client, previousWriter = writer, position = status?.position
        generation = UUID()
        session = nil; status = nil; itemID = nil; client = nil; writer = nil; busy = false; message = nil
        if let position, let previousWriter { _ = try? await previousWriter.update(seconds: position, watched: false) }
        if revoke, let previous, let previousClient { try? await previousClient.endCast(id: previous.id) }
    }
}
