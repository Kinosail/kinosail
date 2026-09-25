import Foundation

/// Decoded, session-only screen values. The protected catalog cache remains the
/// source for launches and background refreshes.
@MainActor
final class ResourceSnapshotCache {
    private struct Key: Hashable {
        let clientID: UUID
        let identity: String
        let type: ObjectIdentifier
    }
    private struct Entry {
        let value: Any
        let refreshedAt: Date?
        let refreshID: String?
    }

    private var values: [Key: Entry] = [:]
    private var order: [Key] = []
    private let limit = 64

    func value<Value>(for identity: String, clientID: UUID, as type: Value.Type = Value.self) -> Value? {
        guard let key = key(identity, clientID: clientID, type: type), let value = values[key]?.value as? Value else { return nil }
        order.removeAll { $0 == key }
        order.append(key)
        return value
    }

    func isFresh<Value>(for identity: String, clientID: UUID, as type: Value.Type = Value.self,
                        refreshID: String, now: Date = Date()) -> Bool {
        guard let key = key(identity, clientID: clientID, type: type), let entry = values[key],
              entry.refreshID == refreshID, let saved = entry.refreshedAt else { return false }
        return (0..<60).contains(now.timeIntervalSince(saved))
    }

    func store<Value>(_ value: Value, for identity: String, clientID: UUID, refreshID: String? = nil) {
        guard let key = key(identity, clientID: clientID, type: Value.self) else { return }
        order.removeAll { $0 == key }
        order.append(key)
        values[key] = Entry(value: value, refreshedAt: refreshID == nil ? nil : Date(), refreshID: refreshID)
        while order.count > limit { values.removeValue(forKey: order.removeFirst()) }
    }

    func remove<Value>(for identity: String, clientID: UUID, as type: Value.Type = Value.self) {
        guard let key = key(identity, clientID: clientID, type: type) else { return }
        values[key] = nil
        order.removeAll { $0 == key }
    }

    func clear() { values.removeAll(); order.removeAll() }

    private func key<Value>(_ identity: String, clientID: UUID, type: Value.Type) -> Key? {
        guard !identity.isEmpty, identity.utf8.count <= 1024 else { return nil }
        return Key(clientID: clientID, identity: identity, type: ObjectIdentifier(type))
    }
}
