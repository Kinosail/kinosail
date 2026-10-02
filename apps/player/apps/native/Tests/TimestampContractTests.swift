import Foundation
import Testing
@testable import KinosailPlayer

struct TimestampContractTests {
    @Test(arguments: [
        ("1970-01-01T00:00:00Z", 0.0),
        ("1970-01-01T00:00:00.123Z", 0.123),
        ("2026-09-30T13:45:10.123456789Z", 1_790_775_910.123),
        ("2026-09-30T13:45:10.999999999Z", 1_790_775_910.999),
        ("1970-01-01T02:30:00+02:30", 0.0),
        ("1969-12-31T20:30:00-03:30", 0.0),
        ("2026-09-30T13:45:10Z", 1_790_775_910.0)
    ])
    func parsesTimestampMeaning(_ raw: String, _ expected: Double) throws {
        #expect(abs(try Input.date(raw).timeIntervalSince1970 - expected) < 0.000_001)
    }

    @Test func acceptsTheServersZeroTimestampInCatalogMetadata() throws {
        let item = JSONValue.object(["id": .string("movie"), "kind": .string("video"),
                                     "title": .string("Movie"), "added": .string("0001-01-01T00:00:00Z"),
                                     "progress": .object(["updated": .string("0001-01-01T00:00:00Z")])])
        #expect(try MediaItem(item, server: ServerAddress("https://fixture.example.invalid")).id == "movie")
    }

    @Test(arguments: ["", "2026-09-30", "2026-09-30T13:45:10", "2026-09-30T13:45:10z",
                      "2026-09-30T13:45:10.Z", "2026-09-30T13:45:10.1234567890Z",
                      "2026-09-30T13:45:10+24:00", "2026-09-30T13:45:10+01:60",
                      "2026-09-30T13:45:10Z trailing", "2026-09-30T13:45:10Z\n",
                      "2026-13-01T00:00:00Z", "2026-09-30T23:59:60Z",
                      String(repeating: "x", count: 41)])
    func rejectsMalformedAndUnboundedTimestamps(_ raw: String) {
        #expect(throws: ClientError.self) { try Input.date(raw) }
    }

    @Test func independentCallersKeepTheirTimezoneAndFractionSettings() async throws {
        let inputs: [(String, Double)] = [
            ("1970-01-01T00:00:00Z", 0),
            ("1970-01-01T00:00:00.123Z", 0.123),
            ("1970-01-01T02:30:00+02:30", 0),
            ("1969-12-31T20:30:00-03:30", 0)
        ]
        try await withThrowingTaskGroup(of: Void.self) { group in
            for worker in 0..<8 {
                group.addTask {
                    for offset in 0..<40 {
                        let (raw, expected) = inputs[(worker + offset) % inputs.count]
                        #expect(throws: ClientError.self) { try Input.date("2026-09-30T23:59:60Z") }
                        let actual = try Input.date(raw).timeIntervalSince1970
                        #expect(abs(actual - expected) < 0.000_001)
                    }
                }
            }
            try await group.waitForAll()
        }
    }
}
