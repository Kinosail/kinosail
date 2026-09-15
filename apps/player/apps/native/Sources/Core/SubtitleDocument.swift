import Foundation

struct SubtitleDocument: Sendable {
    struct Cue: Sendable {
        let start: Double
        let end: Double
        let text: String
    }
    let cues: [Cue]
    private let latestEnd: [Double]

    init(data: Data) throws {
        guard data.count <= 2 * 1024 * 1024, var text = String(data: data, encoding: .utf8), !text.contains("\0") else { throw ClientError.invalidResponse }
        if text.hasPrefix("\u{feff}") { text.removeFirst() }
        text = text.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n")
        let lines = text.components(separatedBy: "\n")
        guard let header = lines.first, header == "WEBVTT" || header.hasPrefix("WEBVTT ") || header.hasPrefix("WEBVTT\t"), lines.count <= 100_000 else { throw ClientError.invalidResponse }
        var cues: [Cue] = []
        var block: [String] = []
        for line in lines.dropFirst() + [""] {
            if line.trimmingCharacters(in: .whitespaces).isEmpty {
                if let cue = try Self.cue(block) { cues.append(cue) }
                guard cues.count <= 20_000 else { throw ClientError.invalidResponse }
                block.removeAll(keepingCapacity: true)
            } else {
                guard line.utf8.count <= 4096, block.count < 64 else { throw ClientError.invalidResponse }
                block.append(line)
            }
        }
        cues.sort { $0.start < $1.start }
        var ends: [Double] = [], prefix: [Double] = [], largest = 0.0
        for cue in cues {
            ends.removeAll { $0 <= cue.start }; ends.append(cue.end)
            guard ends.count <= 32 else { throw ClientError.invalidResponse }
            largest = max(largest, cue.end); prefix.append(largest)
        }
        self.cues = cues; latestEnd = prefix
    }

    func text(at seconds: Double) -> String {
        guard seconds.isFinite, seconds >= 0 else { return "" }
        var low = 0, high = cues.count
        while low < high {
            let middle = (low + high) / 2
            if cues[middle].start <= seconds { low = middle + 1 } else { high = middle }
        }
        var selected: [String] = [], index = low
        while index > 0 {
            index -= 1
            if latestEnd[index] <= seconds { break }
            if cues[index].end > seconds { selected.append(cues[index].text) }
        }
        return selected.reversed().joined(separator: "\n")
    }

    private static func cue(_ block: [String]) throws -> Cue? {
        guard let first = block.first else { return nil }
        if first == "NOTE" || first.hasPrefix("NOTE ") || first.hasPrefix("NOTE\t") || first == "STYLE" || first == "REGION" { return nil }
        let timingIndex = first.contains("-->") ? 0 : 1
        guard block.indices.contains(timingIndex), block.count > timingIndex + 1 else { throw ClientError.invalidResponse }
        let parts = block[timingIndex].components(separatedBy: "-->")
        guard parts.count == 2 else { throw ClientError.invalidResponse }
        let start = try timestamp(parts[0].trimmingCharacters(in: .whitespaces))
        let endField = parts[1].trimmingCharacters(in: .whitespaces).split(whereSeparator: { $0.isWhitespace }).first.map(String.init) ?? ""
        let end = try timestamp(endField)
        guard end > start, block.count - timingIndex - 1 <= 16 else { throw ClientError.invalidResponse }
        var text = block.dropFirst(timingIndex + 1).joined(separator: "\n")
        text = text.replacingOccurrences(of: "<[^>]{0,512}>", with: "", options: .regularExpression)
        for (entity, value) in [("&lt;", "<"), ("&gt;", ">"), ("&nbsp;", " "), ("&lrm;", "\u{200e}"), ("&rlm;", "\u{200f}"), ("&quot;", "\""), ("&apos;", "'"), ("&amp;", "&")] { text = text.replacingOccurrences(of: entity, with: value) }
        guard text.utf8.count <= 4096, !text.unicodeScalars.contains(where: { CharacterSet.controlCharacters.subtracting(.newlines).contains($0) }) else { throw ClientError.invalidResponse }
        return Cue(start: start, end: end, text: text)
    }

    private static func timestamp(_ raw: String) throws -> Double {
        guard raw.range(of: "\\A(?:[0-9]{2,4}:)?[0-5][0-9]:[0-5][0-9]\\.[0-9]{3}\\z", options: .regularExpression) != nil else { throw ClientError.invalidResponse }
        let parts = raw.split(separator: ":").compactMap { Double($0) }
        let seconds = parts.count == 3 ? parts[0] * 3600 + parts[1] * 60 + parts[2] : parts[0] * 60 + parts[1]
        try Input.position(seconds)
        return seconds
    }
}
