#if os(iOS)
import Foundation
import Observation
import OSLog
import UIKit
import UserNotifications

enum DownloadLog {
    private static let log = Logger(subsystem: "com.kinosail.player", category: "downloads")

    static func retry(kind: String, status: Int, attempt: Int) {
        log.warning("operation=download-transfer outcome=retry-scheduled class=\(kind, privacy: .public) code=\(status, privacy: .public) attempt=\(attempt, privacy: .public)")
    }

    static func stopped(reason: String, status: Int) {
        log.error("operation=download-transfer outcome=stopped reason=\(reason, privacy: .public) code=\(status, privacy: .public)")
    }

    static func completed(bytes: Int64) {
        log.info("operation=download-transfer outcome=complete bytes=\(bytes, privacy: .public)")
    }

    static func resumed(count: Int) {
        log.info("operation=download-transfer outcome=retry-now jobs=\(count, privacy: .public)")
    }
}

@MainActor @Observable
final class DownloadNotifications: NSObject, UNUserNotificationCenterDelegate {
    static let shared = DownloadNotifications()
    var openRequested = false
    static let category = "kinosail.downloads"

    func requestAuthorization() async {
        let center = UNUserNotificationCenter.current()
        guard await center.notificationSettings().authorizationStatus == .notDetermined else { return }
        _ = try? await center.requestAuthorization(options: [.alert, .sound])
    }

    nonisolated static func post(id: String, title: String, body: String) {
        Task { @MainActor in
            guard UIApplication.shared.applicationState != .active else { return }
            let center = UNUserNotificationCenter.current()
            let settings = await center.notificationSettings()
            guard [.authorized, .provisional, .ephemeral].contains(settings.authorizationStatus) else { return }
            let content = UNMutableNotificationContent()
            content.title = title
            content.body = body
            content.sound = .default
            content.threadIdentifier = category
            content.userInfo = ["destination": "downloads"]
            try? await center.add(UNNotificationRequest(identifier: id, content: content, trigger: nil))
        }
    }

    nonisolated func userNotificationCenter(_: UNUserNotificationCenter, didReceive response: UNNotificationResponse) async {
        guard response.notification.request.content.userInfo["destination"] as? String == "downloads" else { return }
        await MainActor.run { openRequested = true }
    }

    nonisolated func userNotificationCenter(_: UNUserNotificationCenter, willPresent _: UNNotification) async -> UNNotificationPresentationOptions { [] }
}

extension MediaItem {
    var downloadLabel: String { show.isEmpty ? title : "\(show): \(title)" }
}

extension VerifiedDownloadEngine {
    func notifyReady(_ job: VerifiedDownload) {
        DownloadLog.completed(bytes: job.manifest.size)
        guard let title = job.title, !title.isEmpty else { return }
        DownloadNotifications.post(id: "download-\(job.key)", title: String(localized: "Ready to watch offline"), body: title)
    }

    func notifyAttention(_ job: VerifiedDownload) {
        guard let title = job.title, !title.isEmpty else { return }
        let reason = job.error.isEmpty ? String(localized: "Open Downloads to continue.") : job.error
        DownloadNotifications.post(id: "download-\(job.key)", title: String(localized: "Download needs attention"), body: "\(title). \(reason)")
    }

    func retryNow(_ scope: String) async throws {
        try await ready(scope: scope) {
            var count = 0
            for (id, var job) in self.jobs where job.scope == scope && job.retryAt != nil && job.status != "paused" && job.status != "complete" {
                for (number, value) in self.tasks where value.id == id && value.task.countOfBytesReceived == 0 {
                    self.tasks.removeValue(forKey: number); self.progress.removeValue(forKey: number); value.task.cancel()
                }
                job.retryAt = nil; job.status = "queued"
                try self.persist(job); count += 1
            }
            for (id, var plan) in self.plans where plan.scope == scope && plan.retryAt != nil && plan.status == "preparing" {
                self.cancelPlan(id); plan.retryAt = nil; try self.savePlan(plan); count += 1
            }
            if count > 0 { DownloadLog.resumed(count: count) }
            self.pump()
        }
    }
}
#endif
