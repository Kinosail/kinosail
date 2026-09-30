import Foundation
import ImageIO
import Testing
import Synchronization
import UniformTypeIdentifiers
@testable import KinosailPlayer

final class ControlledArtworkProtocol: URLProtocol, @unchecked Sendable {
    struct Entry: Sendable {
        let data: Data
        var pending: [ControlledArtworkProtocol] = []
        var count = 0
    }
    static let entries = Mutex<[String: Entry]>([:])
    static func active(_ host: String) -> [ControlledArtworkProtocol] { entries.withLock { $0[host]?.pending ?? [] } }
    static func requests(_ host: String) -> Int { entries.withLock { $0[host]?.count ?? 0 } }
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        guard let host = request.url?.host else { return }
        Self.entries.withLock { $0[host]?.pending.append(self); $0[host]?.count += 1 }
    }
    override func stopLoading() {
        guard let host = request.url?.host else { return }
        Self.entries.withLock { $0[host]?.pending.removeAll { $0 === self } }
    }
    func finish() {
        guard let url = request.url, let host = url.host,
              let data = Self.entries.withLock({ $0[host]?.data }),
              let response = HTTPURLResponse(url: url, statusCode: 200, httpVersion: nil, headerFields: ["Content-Type": "image/png"]) else { return }
        stopLoading()
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: data)
        client?.urlProtocolDidFinishLoading(self)
    }
}

func installArtworkImage(_ fixture: HTTPFixture, path: String, width: Int = 400, height: Int = 200) throws {
        installArtwork(fixture, path: path, data: try artworkImageData(width: width, height: height))
    }

func installArtwork(_ fixture: HTTPFixture, path: String, data: Data, type: String = "image/png") {
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes[path] = .init(data: data, status: 200, headers: ["Content-Type": type])
        }
    }

func artworkImageData(width: Int = 400, height: Int = 200, orientation: Int = 1) throws -> Data {
        let context = try #require(CGContext(data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
                                            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.setFillColor(CGColor(red: 0.2, green: 0.8, blue: 0.4, alpha: 0.5))
        context.fill(CGRect(x: 0, y: 0, width: width, height: height))
        let image = try #require(context.makeImage())
        let output = NSMutableData()
        let destination = try #require(CGImageDestinationCreateWithData(output, UTType.png.identifier as CFString, 1, nil))
        CGImageDestinationAddImage(destination, image, [kCGImagePropertyOrientation: orientation] as CFDictionary)
        #expect(CGImageDestinationFinalize(destination))
        return output as Data
    }
