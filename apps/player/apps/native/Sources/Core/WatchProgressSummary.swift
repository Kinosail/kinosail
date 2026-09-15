import Foundation

struct WatchProgressSummary: Sendable {
    let seconds: Double
    let duration: Double
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["seconds", "duration"])
        seconds = try value.requiredNumber("seconds", max: 315_360_000)
        duration = try value.requiredNumber("duration", max: 315_360_000)
        guard duration == 0 || seconds <= duration else { throw ClientError.invalidResponse }
    }
    var fraction: Double? { duration > 0 ? seconds / duration : nil }
    var remainingLabel: String? {
        duration > 0 ? "\(Int(ceil((duration - seconds) / 60))) min left" : nil
    }
}
