import Foundation
@testable import KinosailPlayer

#if os(iOS)
extension BackgroundAuthorizationProbe {
    func recordReauthorizationDelivery(access: DownloadAuthorization, key: String, filePath: String,
                                      revision: UUID, began: ContinuousClock.Instant, initialCount: Int) async {
        // The caller has frozen its failed assertion. Late delivery cannot make it pass.
        let originalFinished = ContinuousClock.now
        let deadline = originalFinished.advanced(by: .seconds(10))
        var interrupted = false
        while fixture.requests.filter({ $0.url?.path == filePath }).count < 2 && ContinuousClock.now < deadline {
            do { try await Task.sleep(for: .milliseconds(5)) }
            catch { interrupted = true; break }
        }
        // Admissions may repeat and include earlier manifest/file requests.
        let delivery = FixtureURLProtocol.entries.withLock { $0[fixture.host] }
        let laterFiles = (delivery?.requests ?? []).filter { $0.url?.path == filePath }
        let milliseconds: (ContinuousClock.Instant) -> Double = { instant in
            let parts = began.duration(to: instant).components
            return Double(parts.seconds) * 1000 + Double(parts.attoseconds) / 1e15
        }
        let state = (try? await engine.snapshot(access.scope))?.first { $0.key == key }
        let diagnostic: [String: Any] = ["initialFileCount": initialCount,
            "initialWaitFinishedMs": milliseconds(originalFinished),
            "diagnosticFinishedMs": milliseconds(.now), "diagnosticInterrupted": interrupted,
            "laterFileCount": laterFiles.count,
            "replacementCredentialsMatch": laterFiles.count == 2
                && laterFiles[1].value(forHTTPHeaderField: "Authorization") == access.header
                && laterFiles[1].value(forHTTPHeaderField: "X-Kinosail-Viewer-Profile") == access.profileID,
            "authorizationAdvanced": await engine.authorizationRevision() != revision,
            "jobStatus": state?.status ?? "unavailable",
            "admissionElapsedMs": Array((delivery?.requestAdmissions ?? []).suffix(16)).map(milliseconds),
            "arrivalElapsedMs": Array((delivery?.requestArrivals ?? []).suffix(16)).map(milliseconds)]
        // No credentials, hostnames, request bodies, or raw errors leave this fixture.
        if let data = try? JSONSerialization.data(withJSONObject: diagnostic, options: [.sortedKeys]),
           let text = String(data: data, encoding: .utf8) { print("BACKGROUND_REAUTH_TIMING \(text)") }
    }
}
#endif
