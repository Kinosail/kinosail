import Foundation
import XCTest
@testable import DownloadRecovery

final class DownloadRecoveryTests: XCTestCase {
  func testPreparedFilesRequireAnExactRouteAndStrongVersion() {
    let id = String(repeating: "a", count: 16), etag = "\"" + String(repeating: "b", count: 64) + "\""
    let url = URL(string: "https://kino.example/api/v1/downloads/" + id + "/file")!
    XCTAssertTrue(DownloadRecovery.validResource(url, etag: etag))
    XCTAssertFalse(DownloadRecovery.validResource(url, etag: nil))
    for invalid in ["", "W/" + etag, "bad", String(repeating: "b", count: 64)] {
      XCTAssertFalse(DownloadRecovery.validResource(url, etag: invalid))
    }
    for path in ["/api/v1/downloads/bad/file", "/api/v1/downloads/" + id, "/admin", "/api/v1/downloads/" + id + "/file?token=x"] {
      XCTAssertFalse(DownloadRecovery.validResource(URL(string: "https://kino.example" + path)!, etag: etag))
    }
    XCTAssertTrue(DownloadRecovery.validResource(URL(string: "https://kino.example/download/arrival")!, etag: nil))
    XCTAssertFalse(DownloadRecovery.validResource(URL(string: "https://kino.example/download/arrival")!, etag: etag))
  }

  private func response(_ status: Int, _ retry: String? = nil) -> HTTPURLResponse {
    HTTPURLResponse(url: URL(string: "https://kino.example/download/title")!, statusCode: status,
      httpVersion: "HTTP/2", headerFields: retry.map { ["Retry-After": $0] })!
  }
  func testTransientResponsesAndExhaustion() {
    for status in [408, 429, 500, 502, 503, 504] {
      XCTAssertEqual(DownloadRecovery.delay(error: nil, response: response(status), attempt: 0), 2)
      XCTAssertEqual(DownloadRecovery.delay(error: nil, response: response(status), attempt: 4), 32)
      XCTAssertNil(DownloadRecovery.delay(error: nil, response: response(status), attempt: 5))
      XCTAssertNil(DownloadRecovery.delay(error: nil, response: response(status), attempt: -1))
    }
  }
  func testServerDelayIsHonoredAndStrictlyBounded() {
    XCTAssertEqual(DownloadRecovery.delay(error: nil, response: response(503, "120"), attempt: 0), 120)
    let now = Date(timeIntervalSince1970: 0)
    XCTAssertEqual(DownloadRecovery.delay(error: nil, response: response(503, "Thu, 01 Jan 1970 00:02:00 GMT"), attempt: 0, now: now), 120)
    for invalid in ["", "-1", "1.5", "soon", "3601", String(repeating: "1", count: 65)] {
      XCTAssertNil(DownloadRecovery.delay(error: nil, response: response(503, invalid), attempt: 0))
    }
  }
  func testDenialsCancellationTLSAndUnknownErrorsDoNotRetry() {
    let disconnected = NSError(domain: NSURLErrorDomain, code: NSURLErrorNetworkConnectionLost)
    for status in [401, 403, 404, 410, 412, 416] {
      XCTAssertNil(DownloadRecovery.delay(error: disconnected, response: response(status), attempt: 0))
    }
    for code in [NSURLErrorCancelled, NSURLErrorServerCertificateUntrusted, NSURLErrorNoPermissionsToReadFile] {
      XCTAssertNil(DownloadRecovery.delay(error: NSError(domain: NSURLErrorDomain, code: code), response: nil, attempt: 0))
    }
    XCTAssertNil(DownloadRecovery.delay(error: NSError(domain: "disk", code: NSURLErrorTimedOut), response: nil, attempt: 0))
    XCTAssertNil(DownloadRecovery.delay(error: nil, response: response(200), attempt: 0))
    XCTAssertNil(DownloadRecovery.delay(error: NSError(domain: NSURLErrorDomain, code: NSURLErrorCancelled), response: response(503), attempt: 0))
  }
  func testConnectionFailuresUseBackoff() {
    for code in [NSURLErrorTimedOut, NSURLErrorCannotFindHost, NSURLErrorCannotConnectToHost,
                 NSURLErrorNetworkConnectionLost, NSURLErrorDNSLookupFailed, NSURLErrorNotConnectedToInternet] {
      XCTAssertEqual(DownloadRecovery.delay(error: NSError(domain: NSURLErrorDomain, code: code), response: nil, attempt: 2), 8)
    }
  }
}
