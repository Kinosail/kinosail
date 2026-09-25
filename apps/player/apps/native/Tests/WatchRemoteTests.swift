import Foundation
import Testing
@testable import KinosailPlayer

struct WatchRemoteTests {
    @Test(arguments: [
        "{}", "[]", #"{"action":"unknown","target":"iphone"}"#,
        #"{"action":"pause","target":"iphone","position":1}"#,
        #"{"action":"seek","target":"iphone"}"#,
        #"{"action":"seek","target":"iphone","position":true}"#,
        #"{"action":"seek","target":"iphone","position":-1}"#,
        #"{"action":"pause","target":"iphone","extra":1}"#,
        #"{"action":"pause","target":""}"#
    ])
    func rejectsMalformedCommands(_ raw: String) {
        #expect(throws: WatchRemoteError.self) { try WatchRemoteRequest.parse(Data(raw.utf8)) }
    }

    @Test func acceptsOnlyBoundedPlayerState() throws {
        let valid = WatchPlayerState(id: "iphone", device: "This iPhone", title: "Film", subtitle: "", itemID: "movie-1",
                                     position: 12, duration: 120, playing: true, audio: false)
        #expect(try valid.validated().title == "Film")
        #expect(throws: WatchRemoteError.self) {
            try WatchPlayerState(id: valid.id, device: valid.device, title: valid.title, subtitle: valid.subtitle,
                                 itemID: valid.itemID, position: 121, duration: 120, playing: true, audio: false).validated()
        }
        #expect(throws: WatchRemoteError.self) {
            try WatchRemoteReply(players: [valid, valid], accepted: true, message: nil).data()
        }
    }

    @Test func mapsReadingsToMovieTimeButLeavesGaps() throws {
        let start = Date(timeIntervalSince1970: 1_000)
        let player = WatchPlayerState(id: "iphone", device: "This iPhone", title: "Film", subtitle: "", itemID: "movie-1",
                                      position: 100, duration: 600, playing: true, audio: false)
        var timeline = MovieHeartTimeline(targetID: player.id, itemID: player.itemID, title: player.title,
                                          duration: player.duration, started: start, ended: nil,
                                          anchors: [HeartAnchor(time: start, position: 100, playing: true)])
        #expect(try timeline.validated().duration == 600)
        #expect(throws: WatchRemoteError.self) {
            try MovieHeartTimeline(targetID: player.id, itemID: player.itemID, title: player.title,
                                   duration: player.duration, started: start, ended: nil,
                                   anchors: [HeartAnchor(time: start, position: 601, playing: true)]).validated()
        }
        #expect(timeline.point(at: start.addingTimeInterval(5), bpm: 95)?.position == 105)
        #expect(timeline.point(at: start.addingTimeInterval(20), bpm: 95) == nil)
        timeline.note(WatchPlayerState(id: player.id, device: player.device, title: player.title, subtitle: "",
                                       itemID: player.itemID, position: 200, duration: 600, playing: false, audio: false),
                      at: start.addingTimeInterval(30))
        #expect(timeline.point(at: start.addingTimeInterval(35), bpm: 100)?.position == 200)
        timeline.note(WatchPlayerState(id: player.id, device: player.device, title: "Next", subtitle: "",
                                       itemID: "movie-2", position: 0, duration: 600, playing: true, audio: false),
                      at: start.addingTimeInterval(40))
        #expect(timeline.ended == start.addingTimeInterval(40))
        #expect(timeline.point(at: start.addingTimeInterval(45), bpm: 100) == nil)
    }
}
