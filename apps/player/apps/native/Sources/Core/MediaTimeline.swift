import Foundation

struct MediaTimeline: Sendable, Equatable {
    let sourceDuration: Double
    let duration: Double
    let omitted: [Range<Double>]

    init(sourceDuration: Double, duration: Double, omitted: [Range<Double>] = []) throws {
        try Input.position(sourceDuration)
        try Input.position(duration)
        guard omitted.count <= 128 else { throw ClientError.invalidResponse }
        var previous: Double = 0
        var removed: Double = 0
        for range in omitted {
            guard range.lowerBound.isFinite, range.upperBound.isFinite, range.lowerBound >= previous,
                  range.upperBound > range.lowerBound, range.upperBound <= sourceDuration else { throw ClientError.invalidResponse }
            previous = range.upperBound
            removed += range.countingLength
        }
        guard abs(sourceDuration - removed - duration) < 0.01 else { throw ClientError.invalidResponse }
        self.sourceDuration = sourceDuration
        self.duration = duration
        self.omitted = omitted
    }

    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["sourceDuration", "duration", "omitted"])
        let source = try value.requiredNumber("sourceDuration")
        let duration = try value.requiredNumber("duration")
        let ranges = try value.list("omitted", max: 128).map { raw in
            let range = try raw.object(allowing: ["start", "end"])
            let start = try range.number("start")
            let end = try range.requiredNumber("end")
            guard end > start else { throw ClientError.invalidResponse }
            return start..<end
        }
        try self.init(sourceDuration: source, duration: duration, omitted: ranges)
    }

    func sourceTime(_ position: Double) -> Double {
        guard position.isFinite else { return 0 }
        let position = min(duration, max(0, position))
        var removed: Double = 0
        for range in omitted {
            if position < range.lowerBound - removed { break }
            removed += range.countingLength
        }
        return min(sourceDuration, position + removed)
    }

    func presentationTime(_ position: Double) -> Double {
        guard position.isFinite else { return 0 }
        let position = min(sourceDuration, max(0, position))
        var removed: Double = 0
        for range in omitted {
            if position < range.lowerBound { break }
            if position < range.upperBound { return range.lowerBound - removed }
            removed += range.countingLength
        }
        return max(0, position - removed)
    }

    var json: JSONValue {
        .object(["sourceDuration": .number(sourceDuration), "duration": .number(duration),
                 "omitted": .array(omitted.map { .object(["start": .number($0.lowerBound), "end": .number($0.upperBound)]) })])
    }
}

private extension Range where Bound == Double {
    var countingLength: Double { upperBound - lowerBound }
}
