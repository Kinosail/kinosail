import Foundation

protocol MediaTransferConsumer: AnyObject {
  func receive(_ response: URLResponse, completion: @escaping (URLSession.ResponseDisposition) -> Void)
  func receive(_ data: Data, task: URLSessionDataTask)
  func complete(_ error: Error?)
}

// One ephemeral connection pool per authorized original-file gateway. Seeking
// reuses TLS/HTTP connections, but never another viewer's credentials or bytes.
// All mutable state and delegate callbacks use the gateway's serial queue.
final class MediaTransferSession: NSObject, URLSessionDataDelegate, @unchecked Sendable {
  private let queue: DispatchQueue
  private var consumers: [Int: MediaTransferConsumer] = [:]
  private var session: URLSession!
  private var closed = false

  init(queue: DispatchQueue, configuration: URLSessionConfiguration = .ephemeral) {
    self.queue = queue
    super.init()
    configuration.urlCredentialStorage = nil
    configuration.urlCache = nil
    configuration.httpCookieStorage = nil
    configuration.httpShouldSetCookies = false
    configuration.httpMaximumConnectionsPerHost = 4
    let delegates = OperationQueue()
    delegates.maxConcurrentOperationCount = 1
    delegates.underlyingQueue = queue
    session = URLSession(configuration: configuration, delegate: self, delegateQueue: delegates)
  }
  // Called on the gateway's serial queue, after method/path/range validation.
  func dataTask(with request: URLRequest, consumer: MediaTransferConsumer) -> URLSessionDataTask? {
    guard !closed, consumers.count < 4 else { return nil }
    let task = session.dataTask(with: request)
    consumers[task.taskIdentifier] = consumer
    return task
  }
  func cancel(_ task: URLSessionDataTask?) {
    guard let task else { return }
    consumers.removeValue(forKey: task.taskIdentifier)
    task.cancel()
  }
  func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
                  newRequest request: URLRequest, completionHandler: @escaping (URLRequest?) -> Void) {
    completionHandler(nil)
  }
  func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive response: URLResponse,
                  completionHandler: @escaping (URLSession.ResponseDisposition) -> Void) {
    guard let consumer = consumers[dataTask.taskIdentifier] else { completionHandler(.cancel); return }
    consumer.receive(response, completion: completionHandler)
  }
  func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
    consumers[dataTask.taskIdentifier]?.receive(data, task: dataTask)
  }
  func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
    consumers.removeValue(forKey: task.taskIdentifier)?.complete(error)
  }
  func close() {
    queue.async { [self] in
      guard !closed else { return }
      closed = true
      session.invalidateAndCancel()
      let active = Array(consumers.values)
      consumers.removeAll()
      active.forEach { $0.complete(URLError(.cancelled)) }
    }
  }
}
