import Foundation
import HealthKit
import Observation

@MainActor @Observable
final class MovieHeartTracker {
    private let health = HKHealthStore()
    private let key = "kinosail.movieHeartTimeline.v1"
    private(set) var timeline: MovieHeartTimeline?
    private(set) var points: [HeartPoint] = []
    private(set) var message: String?
    private(set) var loading = false

    var isTracking: Bool { timeline != nil && timeline?.ended == nil }

    init() {
        if let data = UserDefaults.standard.data(forKey: key), data.count <= 500_000,
           let restored = try? JSONDecoder().decode(MovieHeartTimeline.self, from: data),
           let valid = try? restored.validated() {
            timeline = valid
            if valid.ended == nil && Date().timeIntervalSince(valid.started) > 8 * 3600 {
                timeline?.ended = valid.started.addingTimeInterval(8 * 3600)
                save()
            }
        }
    }

    func matches(_ player: WatchPlayerState) -> Bool {
        timeline?.targetID == player.id && timeline?.itemID == player.itemID
    }

    func start(for player: WatchPlayerState) async {
        guard !isTracking, player.active, !player.audio, player.duration > 0 else { return }
        guard HKHealthStore.isHealthDataAvailable(), let heartRate = HKObjectType.quantityType(forIdentifier: .heartRate) else {
            message = "Heart rate is unavailable on this watch."
            return
        }
        do {
            try await health.requestAuthorization(toShare: [], read: [heartRate])
            let now = Date()
            timeline = MovieHeartTimeline(targetID: player.id, itemID: player.itemID, title: player.title,
                                          duration: player.duration, started: now, ended: nil,
                                          anchors: [HeartAnchor(time: now, position: player.position, playing: player.playing)])
            points = []
            message = nil
            save()
        } catch { message = "Allow heart rate access in Health settings to make a graph." }
    }

    func note(_ player: WatchPlayerState) {
        guard timeline?.targetID == player.id, isTracking else { return }
        timeline?.note(player, at: Date())
        save()
    }

    func stop() async {
        guard isTracking else { return }
        timeline?.ended = Date()
        save()
        await loadSamples()
    }

    func loadSamples() async {
        guard let timeline else { return }
        loading = true
        defer { loading = false }
        do {
            points = try await Self.samples(for: timeline)
            message = points.isEmpty ? "No heart rate samples matched the watched parts of this movie." : nil
        } catch { message = "Heart rate samples could not be loaded from Health." }
    }

    nonisolated private static func samples(for timeline: MovieHeartTimeline) async throws -> [HeartPoint] {
        guard let heartRate = HKObjectType.quantityType(forIdentifier: .heartRate) else { return [] }
        let end = min(timeline.ended ?? Date(), timeline.started.addingTimeInterval(8 * 3600))
        let predicate = HKQuery.predicateForSamples(withStart: timeline.started, end: end, options: .strictStartDate)
        let samples: [HKQuantitySample] = try await withCheckedThrowingContinuation { continuation in
            let store = HKHealthStore()
            let query = HKSampleQuery(sampleType: heartRate, predicate: predicate, limit: 6_000,
                                      sortDescriptors: [NSSortDescriptor(key: HKSampleSortIdentifierEndDate, ascending: true)]) { _, samples, error in
                _ = store
                if let error { continuation.resume(throwing: error) }
                else { continuation.resume(returning: (samples as? [HKQuantitySample]) ?? []) }
            }
            store.execute(query)
        }
        let unit = HKUnit.count().unitDivided(by: .minute())
        return samples.compactMap { timeline.point(at: $0.endDate, bpm: $0.quantity.doubleValue(for: unit)) }
    }

    private func save() {
        if let timeline, let data = try? JSONEncoder().encode(timeline) {
            UserDefaults.standard.set(data, forKey: key)
        }
    }
}
