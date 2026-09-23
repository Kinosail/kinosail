import Foundation

struct SupporterBadge: Identifiable, Sendable {
    let edition: String
    let name: String
    let rank: Int
    let archived: Bool
    var id: String { edition }
    var title: String { ["one-time": "One-time", "monthly": "Monthly", "yearly": "Yearly", "legacy": "Earlier support"][edition] ?? "" }
    var artwork: String { "supporter-\(edition)-\(rank)" }
    init(_ value: JSONValue) throws {
        let object = try value.object(allowing: ["edition", "family", "tier", "name", "rank", "active", "archived"])
        let family = try object.text("family", max: 24, required: true)
        let supplied = try object.text("edition", max: 16)
        edition = supplied.isEmpty ? "legacy" : supplied
        rank = Int(try object.number("rank", max: 10, integer: true))
        name = try object.text("name", max: 40, required: true)
        archived = try object.flag("archived")
        _ = try object.flag("active")
        guard ["one-time", "monthly", "yearly", "legacy"].contains(edition), rank > 0,
              ["patron-order", "living-standard"].contains(family),
              (edition == "one-time") == (family == "patron-order") else { throw ClientError.invalidResponse }
    }
}
struct SupporterCollection: Sendable {
    let badges: [SupporterBadge]
    let visible: Bool
    init(_ value: JSONValue) throws {
        let object = try value.object(allowing: ["badges", "display"])
        guard case .array(let items) = object["badges"], items.count <= 4 else { throw ClientError.invalidResponse }
        badges = try items.map(SupporterBadge.init)
        let display = try object.text("display", max: 24, required: true)
        guard ["automatic", "hidden", "patron-order", "living-standard"].contains(display),
              Set(badges.map(\.edition)).count == badges.count else { throw ClientError.invalidResponse }
        visible = display != "hidden"
    }
}
