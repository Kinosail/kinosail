import Foundation
import Synchronization

/// Accumulate URLSession chunks without a per-byte async iteration. Each task
/// owns its buffer; limits apply before appending, including chunked responses.
final class BoundedHTTPResponse: NSObject, URLSessionDataDelegate, Sendable {
    private typealias Output = (Data, HTTPURLResponse)
    private struct State {
        var data = Data()
        var response: HTTPURLResponse?
        var error: (any Error)?
        var result: Result<Output, any Error>?
        var continuation: CheckedContinuation<Output, any Error>?
    }
    private let state = Mutex(State())
    private let url: URL?
    private let maximum: Int
    private let expected: Set<Int>

    private init(request: URLRequest, maximum: Int, expected: Set<Int>) {
        url = request.url; self.maximum = maximum; self.expected = expected
    }

    static func receive(_ request: URLRequest, session: URLSession, maximum: Int, expected: Set<Int>) async throws -> (Data, HTTPURLResponse) {
        try Task.checkCancellation()
        let receiver = BoundedHTTPResponse(request: request, maximum: maximum, expected: expected)
        let task = session.dataTask(with: request)
        task.delegate = receiver
        return try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { continuation in
                receiver.state.withLock {
                    if let result = $0.result { continuation.resume(with: result) }
                    else { $0.continuation = continuation }
                }
                task.resume()
            }
        } onCancel: { task.cancel() }
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
                    newRequest request: URLRequest, completionHandler: @escaping @Sendable (URLRequest?) -> Void) {
        completionHandler(nil)
    }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive response: URLResponse,
                    completionHandler: @escaping @Sendable (URLSession.ResponseDisposition) -> Void) {
        let allowed = state.withLock { state in
            guard state.result == nil else { return false }
            guard let http = response as? HTTPURLResponse, http.url == url else { state.error = ClientError.invalidResponse; return false }
            guard expected.contains(http.statusCode) else { state.error = ClientError.http(http.statusCode); return false }
            guard http.expectedContentLength <= maximum else { state.error = ClientError.invalidResponse; return false }
            state.response = http
            state.data.reserveCapacity(min(maximum, max(0, Int(http.expectedContentLength))))
            return true
        }
        completionHandler(allowed ? .allow : .cancel)
    }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
        let accepted = state.withLock { state in
            guard state.result == nil, state.error == nil else { return false }
            guard state.response != nil, data.count <= maximum - state.data.count else {
                state.error = ClientError.invalidResponse
                state.data.removeAll(keepingCapacity: false)
                return false
            }
            state.data.append(data)
            return true
        }
        if !accepted { dataTask.cancel() }
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: (any Error)?) {
        state.withLock { state in
            guard state.result == nil else { return }
            let result: Result<Output, any Error>
            if let failure = state.error ?? error { result = .failure(failure) }
            else if let response = state.response { result = .success((state.data, response)) }
            else { result = .failure(ClientError.invalidResponse) }
            state.result = result
            state.data = Data()
            state.continuation?.resume(with: result)
            state.continuation = nil
        }
    }
}
