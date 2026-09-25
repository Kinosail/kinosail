import Foundation

struct HeartAnchor: Codable, Sendable {
    let time: Date
    let position: Double
    let playing: Bool
}

struct HeartPoint: Sendable, Equatable {
    let time: Date
    let position: Double
    let bpm: Double
}

struct MovieHeartTimeline: Codable, Sendable {
    let targetID: String
    let itemID: String
    let title: String
    let duration: Double
    let started: Date
    var ended: Date?
    var anchors: [HeartAnchor]

    func validated() throws -> Self {
        guard (1...64).contains(targetID.utf8.count), (1...128).contains(itemID.utf8.count),
              (1...256).contains(title.utf8.count), duration.isFinite, (0...1_000_000_000).contains(duration),
              started.timeIntervalSince1970.isFinite, ended.map({ $0 >= started }) ?? true,
              !anchors.isEmpty, anchors.count <= 3_000 else { throw WatchRemoteError.invalidMessage }
        var previous = started
        for anchor in anchors {
            guard anchor.time >= previous, anchor.time <= (ended ?? Date()), anchor.position.isFinite,
                  (0...duration).contains(anchor.position) else { throw WatchRemoteError.invalidMessage }
            previous = anchor.time
        }
        return self
    }

    mutating func note(_ player: WatchPlayerState, at time: Date) {
        guard ended == nil, player.id == targetID else { return }
        if player.itemID != itemID { ended = time; return }
        guard anchors.count < 3_000, time >= started else { ended = time; return }
        anchors.append(HeartAnchor(time: time, position: player.position, playing: player.playing))
    }

    func point(at time: Date, bpm: Double) -> HeartPoint? {
        guard time >= started, time <= (ended ?? Date()), (25...250).contains(bpm),
              let anchor = anchors.last(where: { $0.time <= time }), time.timeIntervalSince(anchor.time) <= 15 else { return nil }
        let position = anchor.position + (anchor.playing ? time.timeIntervalSince(anchor.time) : 0)
        guard (0...duration).contains(position) else { return nil }
        return HeartPoint(time: time, position: position, bpm: bpm)
    }
}
