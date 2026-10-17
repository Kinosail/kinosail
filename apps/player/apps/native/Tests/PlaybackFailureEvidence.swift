import AVFoundation
import Foundation
@testable import KinosailPlayer

struct PlaybackSafeFailure: Encodable, Sendable {
    enum Kind: String, Encodable, Sendable {
        case none, cancelled, http, invalidInput, invalidResponse, unavailable
        case secureStorage, discovery, seekUnfinished, avFoundation, url, osStatus, other
    }
    let kind: Kind
    let code: Int?

    init(_ error: Error?) {
        guard let error else { kind = .none; code = nil; return }
        if error is CancellationError { kind = .cancelled; code = nil; return }
        if let client = error as? ClientError {
            switch client {
            case .http(let status):
                kind = .http; code = (100...599).contains(status) ? status : nil
            case .retryAfter: kind = .http; code = 503
            case .invalidInput(let message):
                kind = message == "Could not skip to that position. Try again." ? .seekUnfinished : .invalidInput
                code = nil
            case .invalidResponse: kind = .invalidResponse; code = nil
            case .unavailable: kind = .unavailable; code = nil
            case .secureStorage: kind = .secureStorage; code = nil
            case .discovery: kind = .discovery; code = nil
            }
            return
        }
        let native = error as NSError
        switch native.domain {
        case AVFoundationErrorDomain: kind = .avFoundation
        case NSURLErrorDomain: kind = .url
        case NSOSStatusErrorDomain: kind = .osStatus
        default: kind = .other
        }
        code = kind == .other || !(-2_147_483_648...2_147_483_647).contains(native.code) ? nil : native.code
    }
}

struct PlaybackFailureHTTPReceipt: Encodable, Sendable {
    enum Operation: String, Encodable, Sendable {
        case itemsPlayback, playbackPreferences, media, progressSync
    }
    let operation: Operation
    let requestID: String?
    let status: Int
    let elapsedMilliseconds: Int

    init?(operation: Operation, requestID: String?, status: Int, elapsed: Double) {
        guard (100...599).contains(status), elapsed.isFinite, (0...180).contains(elapsed) else { return nil }
        guard requestID == nil || requestID.flatMap(Self.requestUUID) != nil else { return nil }
        self.operation = operation
        self.requestID = requestID.flatMap(Self.requestUUID)
        self.status = status
        elapsedMilliseconds = Int(elapsed * 1000)
    }

    static func requestUUID(_ input: String) -> String? {
        guard input.utf8.count == 36, let id = UUID(uuidString: input),
              input.uppercased() == id.uuidString else { return nil }
        return id.uuidString
    }
}
