import Foundation

/// Bounded JSON with duplicate-key detection before domain decoding.
enum StrictJSON {
    static func decode(_ data: Data, maximum: Int = 2 * 1024 * 1024) throws -> JSONValue {
        guard !data.isEmpty, data.count <= maximum else { throw ClientError.invalidResponse }
        var parser = Parser(bytes: Array(data))
        let value = try parser.value(depth: 0)
        parser.whitespace()
        guard parser.index == parser.bytes.count else { throw ClientError.invalidResponse }
        return value
    }

    private struct Parser {
        let bytes: [UInt8]
        var index = 0
        var remaining = 100_000

        mutating func whitespace() {
            while index < bytes.count, [9, 10, 13, 32].contains(bytes[index]) { index += 1 }
        }

        mutating func take(_ byte: UInt8) -> Bool {
            whitespace()
            guard index < bytes.count, bytes[index] == byte else { return false }
            index += 1
            return true
        }

        mutating func value(depth: Int) throws -> JSONValue {
            whitespace()
            remaining -= 1
            guard remaining >= 0, depth <= 32, index < bytes.count else { throw ClientError.invalidResponse }
            switch bytes[index] {
            case 123:
                index += 1
                var object: [String: JSONValue] = [:]
                if take(125) { return .object(object) }
                repeat {
                    whitespace()
                    let key = try string()
                    guard key.utf8.count <= 256, object[key] == nil, take(58) else { throw ClientError.invalidResponse }
                    object[key] = try value(depth: depth + 1)
                    if take(125) { return .object(object) }
                } while take(44)
                throw ClientError.invalidResponse
            case 91:
                index += 1
                var array: [JSONValue] = []
                if take(93) { return .array(array) }
                repeat {
                    guard array.count < 16_384 else { throw ClientError.invalidResponse }
                    array.append(try value(depth: depth + 1))
                    if take(93) { return .array(array) }
                } while take(44)
                throw ClientError.invalidResponse
            case 34: return .string(try string())
            case 116: try literal("true"); return .bool(true)
            case 102: try literal("false"); return .bool(false)
            case 110: try literal("null"); return .null
            default:
                let start = index
                while index < bytes.count, ![9, 10, 13, 32, 44, 93, 125].contains(bytes[index]) { index += 1 }
                guard index > start, index - start <= 64,
                      let number = try? JSONDecoder().decode(Double.self, from: Data(bytes[start..<index])), number.isFinite
                else { throw ClientError.invalidResponse }
                return .number(number)
            }
        }

        mutating func literal(_ text: String) throws {
            let expected = Array(text.utf8)
            guard index + expected.count <= bytes.count,
                  Array(bytes[index..<(index + expected.count)]) == expected else { throw ClientError.invalidResponse }
            index += expected.count
        }

        mutating func string() throws -> String {
            guard index < bytes.count, bytes[index] == 34 else { throw ClientError.invalidResponse }
            let start = index
            index += 1
            while index < bytes.count {
                let byte = bytes[index]
                index += 1
                if byte == 92 { index += 1 }
                else if byte == 34 {
                    guard let text = try? JSONDecoder().decode(String.self, from: Data(bytes[start..<index])) else {
                        throw ClientError.invalidResponse
                    }
                    return text
                }
                guard index - start <= 131_072 else { throw ClientError.invalidResponse }
            }
            throw ClientError.invalidResponse
        }
    }
}

extension Dictionary where Key == String, Value == JSONValue {
    func required(_ key: String) throws -> JSONValue {
        guard let value = self[key] else { throw ClientError.invalidResponse }
        return value
    }

    func requiredNumber(_ key: String, max: Double = 31_536_000, integer: Bool = false) throws -> Double {
        _ = try required(key)
        return try number(key, max: max, integer: integer)
    }
}

extension Input {
    static func hex(_ value: String, count: Int) throws -> String {
        guard value.utf8.count == count, value.utf8.allSatisfy({ (48...57).contains($0) || (97...102).contains($0) }) else {
            throw ClientError.invalidInput("The requested identifier is invalid.")
        }
        return value
    }

    static func collection(_ value: String) throws -> String {
        let name = try text(value, max: 64, label: "collection name")
        guard name == value, !name.contains("\\"),
              !name.split(separator: "/", omittingEmptySubsequences: false).contains(where: { $0 == "." || $0 == ".." }) else {
            throw ClientError.invalidInput("The collection name is invalid.")
        }
        return name
    }

    static func segment(_ value: String) -> String {
        value.addingPercentEncoding(withAllowedCharacters: CharacterSet(charactersIn: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~")) ?? ""
    }

    static func unique<T: Identifiable>(_ values: [T]) throws -> [T] where T.ID: Hashable {
        guard Set(values.map(\.id)).count == values.count else { throw ClientError.invalidResponse }
        return values
    }

    static func date(_ value: String) throws -> Date {
        guard value.utf8.count <= 40,
              value.range(of: "\\A[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]{1,9})?(Z|[+-](?:[01][0-9]|2[0-3]):[0-5][0-9])\\z", options: .regularExpression) != nil
        else { throw ClientError.invalidResponse }
        let parser = ISO8601DateFormatter()
        parser.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        guard let date = parser.date(from: value) ?? ISO8601DateFormatter().date(from: value) else { throw ClientError.invalidResponse }
        return date
    }
}
