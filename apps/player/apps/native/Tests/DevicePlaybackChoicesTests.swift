import Foundation
import Testing
@testable import KinosailPlayer

struct DevicePlaybackChoicesTests {
    @Test func remembersSelectionsOnlyForTheCurrentDeviceProfile() {
        let suite = "kinosail.playback.tests.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        var choices = DevicePlaybackChoices()
        choices.rate = 2
        choices.subtitleLanguage = "en"
        choices.subtitleTrack = "English"
        choices.audioLanguage = "es"
        choices.audioTrack = "Spanish"
        choices.save(scope: "profile-a", to: defaults)

        #expect(DevicePlaybackChoices.load(scope: "profile-a", from: defaults) == choices)
        #expect(DevicePlaybackChoices.load(scope: "profile-b", from: defaults) == DevicePlaybackChoices())
        var baseline = PlaybackPreferences()
        baseline.dialogueBoost = true; baseline.nightMode = true
        let applied = DevicePlaybackChoices.load(scope: "profile-a", from: defaults).apply(to: baseline)
        #expect(applied.dialogueBoost && applied.nightMode)
        #expect(applied.rate == 2)
        #expect(applied.subtitleLanguage == "en" && applied.subtitleTrack == "English")
        #expect(applied.audioLanguage == "es" && applied.audioTrack == "Spanish")
    }

    @Test(arguments: [
        "{\"rate\":0.1}", "{\"rate\":4}", "{\"rate\":\"2\"}", "{\"rate\":2,\"rate\":3}",
        "{\"subtitleLanguage\":\"other\"}", "{\"subtitleLanguage\":\"off\",\"subtitleTrack\":true}",
        "{\"subtitleTrack\":\"English\"}", "{\"subtitleLanguage\":\"off\",\"subtitleTrack\":\"English\"}",
        "{\"audioLanguage\":\"123\"}", "{\"unknown\":true}"
    ])
    func rejectsInvalidSavedChoicesWithoutChangingPlayback(_ raw: String) {
        let suite = "kinosail.playback.tests.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        defaults.set(Data(raw.utf8), forKey: DevicePlaybackChoices.key(scope: "profile"))
        let baseline = PlaybackPreferences()
        #expect(DevicePlaybackChoices.load(scope: "profile", from: defaults).apply(to: baseline) == baseline)
        #expect(defaults.data(forKey: DevicePlaybackChoices.key(scope: "profile")) == Data(raw.utf8))
    }

    @Test func remembersExplicitSubtitleOffAndRejectsOversizedState() {
        let suite = "kinosail.playback.tests.\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        var choices = DevicePlaybackChoices()
        choices.subtitleLanguage = "off"
        choices.subtitleTrack = ""
        choices.save(scope: "profile", to: defaults)
        #expect(DevicePlaybackChoices.load(scope: "profile", from: defaults).apply(to: PlaybackPreferences()).subtitleLanguage == "off")
        defaults.set(Data(repeating: 65, count: 1025), forKey: DevicePlaybackChoices.key(scope: "profile"))
        #expect(DevicePlaybackChoices.load(scope: "profile", from: defaults) == DevicePlaybackChoices())
    }
}
