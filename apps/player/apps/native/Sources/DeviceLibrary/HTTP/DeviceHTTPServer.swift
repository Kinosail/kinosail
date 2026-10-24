import CryptoKit
import Foundation
import Network
import OSLog

let deviceLibraryLog = Logger(subsystem: "com.kinosail.player", category: "device-library")

final class DeviceHTTPServer: @unchecked Sendable {
    static let connectionLimit = 48
    private let queue = DispatchQueue(label: "com.kinosail.player.device-library")
    private let token: String
    private let handler: DeviceRequestHandler
    private var listener: NWListener?
    private var transfer: MediaTransferSession?
    private var connections: [UUID: DeviceHTTPConnection] = [:]
    private(set) var port: UInt16 = 0

    init(token: String, handler: DeviceRequestHandler) {
        self.token = token
        self.handler = handler
    }

    func start(preferredPort: UInt16) async throws -> UInt16 {
        if preferredPort != 0, let bound = try? await bind(preferredPort) { return bound }
        return try await bind(0)
    }

    private func bind(_ candidate: UInt16) async throws -> UInt16 {
        let parameters = NWParameters.tcp
        parameters.allowLocalEndpointReuse = true
        let endpointPort = candidate == 0 ? NWEndpoint.Port.any : NWEndpoint.Port(rawValue: candidate) ?? .any
        parameters.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: endpointPort)
        let next = try NWListener(using: parameters)
        return try await withCheckedThrowingContinuation { continuation in
            queue.async { [self] in
                let gate = DeviceOnce()
                next.stateUpdateHandler = { [weak self] state in
                    guard !gate.done else {
                        if case .failed = state { self?.listenerFailed(next) }
                        return
                    }
                    switch state {
                    case .ready:
                        gate.done = true
                        guard let port = next.port?.rawValue else { continuation.resume(throwing: ClientError.unavailable); return }
                        self?.listener?.cancel()
                        self?.listener = next
                        self?.port = port
                        if self?.transfer == nil, let queue = self?.queue { self?.transfer = MediaTransferSession(queue: queue) }
                        continuation.resume(returning: port)
                    case .failed, .cancelled:
                        gate.done = true
                        next.cancel()
                        continuation.resume(throwing: ClientError.unavailable)
                    default: break
                    }
                }
                next.newConnectionHandler = { [weak self] connection in self?.accept(connection) }
                next.start(queue: queue)
            }
        }
    }

    private func listenerFailed(_ failed: NWListener) {
        guard listener === failed else { return }
        deviceLibraryLog.error("operation=device-library-listen outcome=failed")
        listener = nil
    }

    var running: Bool {
        queue.sync { listener != nil }
    }

    private func accept(_ connection: NWConnection) {
        guard let transfer, connections.count < Self.connectionLimit else {
            deviceLibraryLog.warning("operation=device-library-accept outcome=rejected failure_class=capacity")
            connection.cancel(); return
        }
        let id = UUID()
        let stream = DeviceHTTPConnection(connection: connection, queue: queue, transfer: transfer, port: port,
                                          authorized: { [token] value in Self.authorized(value, token: token) },
                                          handler: handler) { [weak self] in self?.connections.removeValue(forKey: id) }
        connections[id] = stream
        stream.start()
    }

    static func authorized(_ header: String?, token: String) -> Bool {
        guard let header, header.hasPrefix("Bearer ") else { return false }
        let presented = Data(header.dropFirst(7).utf8), expected = Data(token.utf8)
        return SHA256.hash(data: presented) == SHA256.hash(data: expected)
    }

    func stop() {
        queue.async { [self] in
            listener?.cancel(); listener = nil
            let active = Array(connections.values)
            connections.removeAll()
            active.forEach { $0.close() }
            transfer?.close(); transfer = nil
        }
    }
}

private final class DeviceOnce: @unchecked Sendable {
    var done = false
}
