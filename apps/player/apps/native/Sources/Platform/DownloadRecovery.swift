import Foundation

// Shared by the delegate and focused native tests. Delays are OS-scheduled,
// so retrying does not require a JavaScript timer or keeping the app awake.
enum DownloadRecovery {
  static func delay(error: NSError?, response: HTTPURLResponse?, attempt: Int,
                    now: Date = Date(), jitter: Double = Double.random(in: 0...1)) -> TimeInterval? {
    guard jitter.isFinite, (0...1).contains(jitter), (0..<5).contains(attempt) else { return nil }
    let temporaryStatus = [408, 429, 500, 502, 503, 504].contains(response?.statusCode ?? 0)
    let temporaryNetwork = error?.domain == NSURLErrorDomain && [
      NSURLErrorTimedOut, NSURLErrorCannotFindHost, NSURLErrorCannotConnectToHost,
      NSURLErrorNetworkConnectionLost, NSURLErrorDNSLookupFailed,
      NSURLErrorNotConnectedToInternet
    ].contains(error?.code ?? 0)
    if error != nil && !temporaryNetwork { return nil }
    // An actual denial or changed resource must not be hidden by a network error.
    if let status = response?.statusCode, status >= 400 && !temporaryStatus { return nil }
    guard temporaryStatus || temporaryNetwork else { return nil }
    var delay = min(60, pow(2, Double(attempt + 1)))
    if let raw = response?.value(forHTTPHeaderField: "Retry-After") {
      guard raw.utf8.count <= 64 else { return nil }
      let seconds: TimeInterval?
      if !raw.isEmpty && raw.utf8.allSatisfy({ (48...57).contains($0) }) {
        seconds = TimeInterval(raw)
      } else {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = TimeZone(secondsFromGMT: 0)
        formatter.dateFormat = "EEE, dd MMM yyyy HH:mm:ss 'GMT'"
        formatter.isLenient = false
        let date = formatter.date(from: raw)
        seconds = date.flatMap { formatter.string(from: $0) == raw ? $0.timeIntervalSince(now) : nil }
      }
      guard let seconds, seconds.isFinite, seconds <= 3600 else { return nil }
      delay = max(delay, seconds)
    }
    return delay + min(30, delay * 0.25) * jitter
  }
}
