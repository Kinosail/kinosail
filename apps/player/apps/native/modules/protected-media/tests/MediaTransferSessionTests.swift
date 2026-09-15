import Foundation
import XCTest
@testable import DownloadRecovery

private final class MediaStub: URLProtocol {
  override class func canInit(with request: URLRequest) -> Bool { true }
  override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
  override func startLoading() {
    let data = Data((request.value(forHTTPHeaderField: "Range") ?? "missing-range").utf8)
    let response = HTTPURLResponse(url: request.url!, statusCode: 206, httpVersion: "HTTP/1.1", headerFields: ["Content-Length": "\(data.count)"])!
    client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
    client?.urlProtocol(self, didLoad: data)
    client?.urlProtocolDidFinishLoading(self)
  }
  override func stopLoading() {}
}
private final class MediaSink: MediaTransferConsumer {
  var bytes = Data()
  let done: XCTestExpectation
  init(_ done: XCTestExpectation) { self.done = done }
  func receive(_ response: URLResponse, completion: @escaping (URLSession.ResponseDisposition) -> Void) { completion(.allow) }
  func receive(_ data: Data, task: URLSessionDataTask) { bytes.append(data) }
  func complete(_ error: Error?) { done.fulfill() }
}
final class MediaTransferSessionTests: XCTestCase {
  func testConcurrentRangesHaveIndependentConsumersAndReuseTheSession() {
    let queue = DispatchQueue(label: "media-test")
    let configuration = URLSessionConfiguration.ephemeral
    configuration.protocolClasses = [MediaStub.self]
    let transfer = MediaTransferSession(queue: queue, configuration: configuration)
    defer { transfer.close() }
    let first = MediaSink(expectation(description: "first range"))
    let second = MediaSink(expectation(description: "second range"))
    queue.sync {
      var request = URLRequest(url: URL(string: "https://media.invalid/media/movie")!)
      request.setValue("bytes=0-99", forHTTPHeaderField: "Range")
      let one = transfer.dataTask(with: request, consumer: first)!
      request.setValue("bytes=100-199", forHTTPHeaderField: "Range")
      let two = transfer.dataTask(with: request, consumer: second)!
      XCTAssertNotEqual(one.taskIdentifier, two.taskIdentifier)
      one.resume(); two.resume()
    }
    wait(for: [first.done, second.done], timeout: 2)
    queue.sync {
      XCTAssertEqual(String(data: first.bytes, encoding: .utf8), "bytes=0-99")
      XCTAssertEqual(String(data: second.bytes, encoding: .utf8), "bytes=100-199")
    }
  }
  func testBoundedTasksAndClosedGatewayCannotStartMoreTransfers() {
    let queue = DispatchQueue(label: "media-limit-test")
    let transfer = MediaTransferSession(queue: queue)
    let request = URLRequest(url: URL(string: "https://media.invalid/media/movie")!)
    let closed = expectation(description: "cancel every task")
    closed.expectedFulfillmentCount = 4
    queue.sync {
      let first = transfer.dataTask(with: request, consumer: MediaSink(closed))!
      for _ in 0..<3 { XCTAssertNotNil(transfer.dataTask(with: request, consumer: MediaSink(closed))) }
      XCTAssertNil(transfer.dataTask(with: request, consumer: MediaSink(closed)))
      transfer.cancel(first)
      XCTAssertNotNil(transfer.dataTask(with: request, consumer: MediaSink(closed)))
    }
    transfer.close()
    wait(for: [closed], timeout: 2)
    queue.sync { XCTAssertNil(transfer.dataTask(with: request, consumer: MediaSink(closed))) }
  }
}
