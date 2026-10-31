import Testing
@testable import KinosailPlayer

struct SourceSurroundAudioTests {
    private func transcodeAudio(_ query: [String: String]) throws -> [String] {
        let profile = JellyfinPlayback.profile(DeviceCapabilities(query: query))
        let profiles = try #require(profile["TranscodingProfiles"] as? [[String: Any]])
        let video = try #require(profiles.first { $0["Type"] as? String == "Video" })
        return try #require(video["AudioCodec"] as? String).split(separator: ",").map(String.init)
    }

    @Test func surroundOutputsAskJellyfinAndEmbyForEAC3First() throws {
        let codecs = try transcodeAudio(["audioCodecs": "aac,ac3,eac3", "maxAudioChannels": "6"])
        #expect(codecs.first == "eac3")
        #expect(Set(codecs) == ["eac3", "ac3", "aac"])
    }

    @Test func stereoOutputsKeepAACFirst() throws {
        #expect(try transcodeAudio(["audioCodecs": "aac,ac3,eac3", "maxAudioChannels": "2"]).first == "aac")
    }

    @Test func clientsWithoutEAC3KeepAACFirst() throws {
        #expect(try transcodeAudio(["audioCodecs": "aac,ac3", "maxAudioChannels": "8"]).first == "aac")
    }

    @Test func plexReceivesTheSameOrder() {
        let surround = DeviceCapabilities(query: ["audioCodecs": "aac,ac3,eac3", "maxAudioChannels": "6"])
        let stereo = DeviceCapabilities(query: ["audioCodecs": "aac,ac3,eac3", "maxAudioChannels": "2"])
        #expect(surround.convertedAudioCodecs == ["eac3", "ac3", "aac"])
        #expect(stereo.convertedAudioCodecs == ["aac", "ac3", "eac3"])
    }
}
