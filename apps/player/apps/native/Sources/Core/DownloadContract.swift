import CryptoKit
import Foundation

extension DownloadTrackSelection {
    func validated() throws -> Self {
        guard audio.count <= 32, subtitles.count <= 256 else { throw ClientError.invalidInput("Too many download tracks were selected.") }
        for values in [audio, subtitles] {
            guard Set(values).count == values.count, values.allSatisfy({ (0...4095).contains($0) }) else { throw ClientError.invalidInput("The download track selection is invalid.") }
        }
        return self
    }
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["audio", "subtitles"])
        func indices(_ key: String, maximum: Int) throws -> [Int] {
            guard case .array(let entries) = try value.required(key), entries.count <= maximum else { throw ClientError.invalidResponse }
            return try entries.map { raw in
                guard case .number(let number) = raw, number.isFinite, number.rounded() == number, (0...4095).contains(number) else { throw ClientError.invalidResponse }
                return Int(number)
            }
        }
        self.init(audio: try indices("audio", maximum: 32), subtitles: try indices("subtitles", maximum: 256))
        _ = try validated()
    }
    var json: JSONValue { .object(["audio": .array(audio.map { .number(Double($0)) }), "subtitles": .array(subtitles.map { .number(Double($0)) })]) }
}

extension PreparedDownload {
    init(_ raw: JSONValue, profileID: String?) throws {
        let value = try raw.object(allowing: ["sourceVersion", "tracks", "id", "itemId", "profileId", "title", "quality", "state", "sha256", "size", "readyOffline", "error", "extension", "created"])
        id = try Input.hex(value.text("id", max: 16, required: true), count: 16)
        itemID = try Input.id(value.text("itemId", max: 128, required: true))
        let profile = try Input.id(value.text("profileId", max: 128, required: true))
        guard profileID == nil || profile == profileID,
              let quality = try DownloadQuality(rawValue: value.text("quality", max: 32, required: true)),
              let state = try State(rawValue: value.text("state", max: 32, required: true)) else { throw ClientError.invalidResponse }
        self.quality = quality; self.state = state
        size = Int64(try value.number("size", max: Double(OfflineManifest.blockSize * 16_384), integer: true))
        sha256 = try value.text("sha256", max: 64)
        if !sha256.isEmpty { _ = try Input.hex(sha256, count: 64) }
        let error = try value.text("error", max: 1000)
        self.error = error.isEmpty ? nil : "The Server could not prepare this download. Check its download status and try again."
        let ready = try value.flag("readyOffline")
        switch state {
        case .preparing: guard !ready, error.isEmpty, sha256.isEmpty, size == 0 else { throw ClientError.invalidResponse }
        case .ready: guard ready, error.isEmpty, !sha256.isEmpty, size > 0 else { throw ClientError.invalidResponse }
        case .failed: guard !ready, !error.isEmpty else { throw ClientError.invalidResponse }
        }
        _ = try value.text("title", max: 512, required: true)
        _ = try value.text("sourceVersion", max: 128)
        _ = try Input.date(value.text("created", max: 40, required: true))
        if let tracks = value["tracks"] { _ = try DownloadTrackSelection(tracks) }
        let ext = try value.text("extension", max: 16)
        guard ext.isEmpty || ext.range(of: "\\A\\.[A-Za-z0-9]{1,15}\\z", options: .regularExpression) != nil else { throw ClientError.invalidResponse }
    }
}

extension OfflineManifest {
    static let blockSize: Int64 = 8 * 1024 * 1024
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["version", "id", "size", "sha256", "chunkSize", "chunks"])
        version = Int(try value.requiredNumber("version", max: 1, integer: true))
        id = try Input.hex(value.text("id", max: 16, required: true), count: 16)
        size = Int64(try value.requiredNumber("size", max: Double(Self.blockSize * 16_384), integer: true))
        sha256 = try Input.hex(value.text("sha256", max: 64, required: true), count: 64)
        chunkSize = Int64(try value.requiredNumber("chunkSize", max: Double(Self.blockSize), integer: true))
        chunks = try value.required("chunks").array(max: 16_384).map { raw in
            guard case .string(let digest) = raw else { throw ClientError.invalidResponse }
            return try Input.hex(digest, count: 64)
        }
        try validate()
    }
    func validate() throws {
        guard version == 1, size > 0, size <= Self.blockSize * 16_384, chunkSize == Self.blockSize,
              chunks.count == Int((size + chunkSize - 1) / chunkSize), Self.digest(sha256), chunks.allSatisfy(Self.digest) else { throw ClientError.invalidResponse }
        _ = try Input.hex(id, count: 16)
    }
    static func digest(_ value: String) -> Bool { (try? Input.hex(value, count: 64)) != nil }
    func length(_ index: Int) -> Int64 { min(chunkSize, size - Int64(index) * chunkSize) }
    var json: JSONValue {
        .object(["version": .number(Double(version)), "id": .string(id), "size": .number(Double(size)), "sha256": .string(sha256),
                 "chunkSize": .number(Double(chunkSize)), "chunks": .array(chunks.map(JSONValue.string))])
    }
}

struct DownloadAuthorization: Sendable {
    let scope: String
    let server: ServerAddress
    let profileID: String
    let header: String

    init(server: ServerAddress, serverID: String, profileID: String, token: String) throws {
        self.server = try ServerAddress(server.url.absoluteString)
        self.profileID = try Input.id(profileID)
        let serverID = try Input.text(serverID, max: 256, label: "Server identity")
        header = "Bearer " + (try Input.secret(token))
        scope = Self.hash(Data("\(server.url.absoluteString)\n\(serverID)\n\(profileID)".utf8))
    }
    static func hash(_ data: Data) -> String { SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined() }
}
