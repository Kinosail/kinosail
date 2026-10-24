import Foundation

struct DeviceLibraryPlayback {
    let catalog: DeviceCatalog
    let state: DeviceLocalState
    let streams: DeviceStreamRegistry

    func playback(_ id: String, request: DeviceRequest) async throws -> DeviceResponse {
        guard let item = try await catalog.lookup(id), item.kind != .series else { throw DeviceFailure.notFound }
        let capabilities = DeviceCapabilities(query: request.query)
        let start = item.state.watched ? 0 : item.state.seconds
        var candidates: [(info: DevicePlaybackInfo, local: Bool)] = []
        var failure: Error?
        for version in (item.versions.isEmpty ? [item.ref] : item.versions).prefix(4) {
            do {
                let client = try await catalog.client(for: version)
                let info = try await client.playback(version.item, capabilities: capabilities, start: start)
                candidates.append((info, client.account.address.host.map(ServerAddress.localHost) ?? false))
            } catch is CancellationError { throw CancellationError() }
            catch { failure = error }
        }
        guard let info = DevicePlayback.choose(candidates) else { throw failure ?? DeviceFailure.notFound }
        var compatible: String?
        if let transcode = info.transcode, let url = transcode.url {
            compatible = try streams.register(url, itemID: id, template: transcode)
        }
        guard info.directPlayable || compatible != nil else {
            deviceLibraryLog.warning("operation=device-library-playback outcome=rejected failure_class=no-playable-stream")
            throw ClientError.http(415)
        }
        var subtitles: [Int: URLRequest] = [:]
        for stream in info.subtitles {
            guard let url = stream.deliveryURL else { continue }
            var request = info.direct
            request.url = url
            subtitles[stream.index] = request
        }
        await state.begin(id, .init(ref: info.ref, session: info.session, direct: info.directPlayable ? info.direct : nil,
                                     subtitles: subtitles, seconds: start))
        let next = try? await nextEpisode(after: item)
        deviceLibraryLog.info("operation=device-library-playback outcome=prepared mode=\(info.directPlayable ? "direct" : "conversion", privacy: .public)")
        return .json(200, DevicePlayback.json(item: item, info: info, start: start, next: next, compatiblePath: compatible))
    }

    func sync(_ id: String, request: DeviceRequest) async throws -> DeviceResponse {
        let (update, expected) = try DeviceLibraryRouter.body(request) { raw -> (WatchProgress, WatchProgress) in
            let value = try raw.object(allowing: ["progress", "expected", "playbackToken"])
            guard let progress = value["progress"], let expected = value["expected"] else { throw DeviceFailure.invalid }
            return (try WatchProgress(progress).validated(required: true), try WatchProgress(expected).validated(required: false))
        }
        guard var item = try await catalog.lookup(id), item.kind != .series else { throw DeviceFailure.notFound }
        let mark = await state.mark(id)
        let current = DeviceProjection.progress(item.state, mark: mark)
        if mark.session == update.session, mark.revision >= update.revision { return .json(200, current) }
        if mark.session != update.session, mark.session != expected.session || mark.revision != expected.revision {
            return .json(409, .object(["error": .string("Progress changed on another device."), "progress": current]))
        }
        let duration = max(item.duration, 1)
        let watched = update.watched || update.seconds >= duration * 0.92
        let seconds = watched ? 0 : update.seconds
        if let playback = await state.playback(id) {
            let client = try await catalog.client(for: playback.ref)
            try await client.report(playback.ref.item, seconds: seconds, watched: watched,
                                    event: playback.started ? .progress : .start, session: playback.session)
            await state.reported(id, seconds: seconds)
        } else {
            try await setPosition(item, seconds: seconds, watched: watched)
        }
        await state.record(id, ProgressMark(session: update.session, revision: update.revision))
        await catalog.updateState(id) { $0.seconds = seconds; $0.watched = watched; $0.lastPlayed = Date() }
        item.state.seconds = seconds
        item.state.watched = watched
        return .json(200, DeviceProjection.progress(item.state, mark: ProgressMark(session: update.session, revision: update.revision)))
    }

    func event(_ id: String, request: DeviceRequest) async throws -> DeviceResponse {
        let kind = try DeviceLibraryRouter.body(request) { raw -> String in
            let value = try raw.object(allowing: ["session", "event", "sequence"])
            _ = try value.text("session", max: 128)
            _ = try value.number("sequence", max: 1_000_000, integer: true)
            return try value.text("event", max: 32, required: true)
        }
        guard kind == "session-end" else { return .empty(204) }
        if let playback = await state.end(id), playback.started {
            let client = try await catalog.client(for: playback.ref)
            try await client.report(playback.ref.item, seconds: playback.seconds, watched: false, event: .stop, session: playback.session)
        }
        return .empty(204)
    }

    func dismiss(_ id: String) async throws -> DeviceResponse {
        guard let item = try await catalog.lookup(id), item.kind != .series else { throw DeviceFailure.notFound }
        try await setPosition(item, seconds: 0, watched: item.state.watched)
        await catalog.updateState(id) { $0.seconds = 0; $0.lastPlayed = nil }
        return .empty(204)
    }

    private func setPosition(_ item: DeviceItem, seconds: Double, watched: Bool) async throws {
        for version in item.versions.isEmpty ? [item.ref] : item.versions {
            try await catalog.client(for: version).setPosition(version.item, seconds: seconds, watched: watched)
        }
    }

    private func nextEpisode(after item: DeviceItem) async throws -> String? {
        guard item.kind == .episode, let key = item.showKey else { return nil }
        let (_, episodes) = try await catalog.show(DeviceItem.identifier(key))
        guard let index = episodes.firstIndex(where: { $0.id == item.id }), index + 1 < episodes.count else { return nil }
        return episodes[(index + 1)...].first { $0.season > 0 }?.id
    }
}
