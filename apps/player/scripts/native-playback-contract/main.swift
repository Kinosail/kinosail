import Foundation

@main struct PlaybackContractJourney {
    static func main() async throws {
        let server = try ServerAddress(CommandLine.arguments[1])
        let output = URL(fileURLWithPath: CommandLine.arguments[2])
        func get(_ path: String) async throws -> Data {
            let (data, response) = try await URLSession.shared.data(from: server.mediaURL(path))
            guard (response as? HTTPURLResponse)?.statusCode == 200, data.count <= 2 * 1024 * 1024 else { throw ClientError.invalidResponse }
            return data
        }
        let catalog = try StrictJSON.decode(await get("/api/v1/library")).object()
        let item = try catalog.required("items").array(max: 1).first!.object()
        let id = try Input.id(item.text("id", required: true))
        let data = try await get("/api/v1/items/\(id)/playback?videoCodecs=h264&audioCodecs=aac&hdrFormats=sdr&maxAudioChannels=2")
        try data.write(to: output.appendingPathComponent("server-response.json"))
        let raw = try StrictJSON.decode(data)
        let original = try raw.object()
        var results: [[String: JSONValue]] = []
        func check(_ name: String, _ raw: JSONValue, accepts: Bool) {
            var accepted = false
            var errorClass = ""
            do { _ = try PlaybackSource(raw, itemID: id, server: server); accepted = true }
            catch { errorClass = error as? ClientError == .invalidResponse ? "invalidResponse" : "other" }
            results.append(["case": .string(name), "expectedAcceptance": .bool(accepts), "accepted": .bool(accepted),
                            "passed": .bool(accepted == accepts), "errorClass": .string(errorClass)])
        }
        check("real-server-response", raw, accepts: true)
        var legacy = original; legacy.removeValue(forKey: "policy")
        check("legacy-policy-omitted", .object(legacy), accepts: true)
        for policy in ["automatic", "direct", "compatible"] {
            var changed = original; changed["policy"] = .string(policy)
            check("policy-\(policy)", .object(changed), accepts: true)
        }
        for (name, value) in [("unknown", JSONValue.string("other")), ("empty", .string("")), ("number", .number(1)),
                              ("boolean", .bool(true)), ("null", .null), ("oversized", .string(String(repeating: "a", count: 33)))] {
            var changed = original; changed["policy"] = value
            check("reject-policy-\(name)", .object(changed), accepts: false)
        }
        var unknown = original; unknown["unexpected"] = .bool(true)
        check("reject-unknown-field", .object(unknown), accepts: false)
        var wrongItem = original; wrongItem["direct"] = .string("/media/other")
        check("reject-wrong-item", .object(wrongItem), accepts: false)
        var foreign = original; foreign["direct"] = .string("https://other.example/media/\(id)")
        check("reject-foreign-origin", .object(foreign), accepts: false)
        var malformedSubtitle = original; malformedSubtitle["subtitlePickerLimited"] = .string("true")
        check("reject-subtitle-type", .object(malformedSubtitle), accepts: false)
        var duplicateRejected = false
        do { _ = try StrictJSON.decode(Data("{\"policy\":\"automatic\",\"policy\":\"direct\"}".utf8)) }
        catch { duplicateRejected = true }
        results.append(["case": .string("reject-duplicate-field"), "passed": .bool(duplicateRejected)])
        var mediaFetched = false
        if let source = try? PlaybackSource(raw, itemID: id, server: server), let direct = source.direct {
            let (media, response) = try await URLSession.shared.data(from: direct)
            mediaFetched = (response as? HTTPURLResponse)?.statusCode == 200 && !media.isEmpty && media.count <= 1024 * 1024
        }
        let artifact = JSONValue.object(["cases": .array(results.map(JSONValue.object)), "mediaFetchedAfterValidation": .bool(mediaFetched)])
        try JSONEncoder().encode(artifact).write(to: output.appendingPathComponent("native-result.json"))
        let failures = results.filter { $0["passed"] != .bool(true) }.count
        print("Native playback contract: \(results.count - failures)/\(results.count) passed; media fetched after validation: \(mediaFetched)")
        if failures > 0 || !mediaFetched { exit(1) }
    }
}
