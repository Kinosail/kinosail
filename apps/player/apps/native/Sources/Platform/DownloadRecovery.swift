import Foundation

enum DownloadRecovery {
  static let attempts = 5
  static let busy = "The Server is busy. Kinosail will try again automatically."
  static let unreachable = "Can’t reach your Server. Kinosail will try again automatically."
  static let signIn = "Sign in again to continue this download."

  static func delay(error: NSError?, response: HTTPURLResponse?, attempt: Int,
                    now: Date = Date(), jitter: Double = Double.random(in: 0...1)) -> TimeInterval? {
    guard jitter.isFinite, (0...1).contains(jitter), (0..<attempts).contains(attempt) else { return nil }
    let temporaryStatus = [408, 429, 500, 502, 503, 504].contains(response?.statusCode ?? 0)
    let temporaryNetwork = transient(error)
    if error != nil && !temporaryNetwork { return nil }
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

  static func retry(error: NSError?, response: HTTPURLResponse?, retries: Int) -> TimeInterval? {
    delay(error: error, response: response, attempt: min(max(retries, 0), attempts - 1))
  }

  static func message(response: HTTPURLResponse?) -> String {
    if let status = response?.statusCode, [429, 503].contains(status) { return busy }
    return unreachable
  }

  static func transient(_ error: NSError?) -> Bool {
    guard let error, error.domain == NSURLErrorDomain else { return false }
    return [
      NSURLErrorTimedOut, NSURLErrorCannotFindHost, NSURLErrorCannotConnectToHost,
      NSURLErrorNetworkConnectionLost, NSURLErrorDNSLookupFailed, NSURLErrorNotConnectedToInternet,
      NSURLErrorInternationalRoamingOff, NSURLErrorCallIsActive, NSURLErrorDataNotAllowed,
      NSURLErrorBackgroundSessionWasDisconnected
    ].contains(error.code)
  }
}
