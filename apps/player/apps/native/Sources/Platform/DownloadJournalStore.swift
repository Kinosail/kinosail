#if os(iOS)
  import Foundation
  import UIKit

  /// Owns protected journals, path confinement, quarantine and durable deletion.
  /// Called only on the download engine's serial queue.
  struct DownloadJournalStore: Sendable {
    private let root: URL

    init(directory: URL? = nil) {
      root = directory?.resolvingSymlinksInPath() ?? FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].resolvingSymlinksInPath().appendingPathComponent("kinosail-swift-offline", isDirectory: true)
    }

    func load() throws -> (jobs: [String: VerifiedDownload], plans: [String: DownloadPreparation]) {
      var jobs: [String: VerifiedDownload] = [:]
      var plans: [String: DownloadPreparation] = [:]
      var metadataBytes = 0
      let scopes = FileManager.default.fileExists(atPath: root.path)
        ? try FileManager.default.contentsOfDirectory(at: root, includingPropertiesForKeys: nil) : []
      for scope in scopes where OfflineManifest.digest(scope.lastPathComponent) {
        guard scope.resolvingSymlinksInPath().standardizedFileURL == scope.standardizedFileURL,
              let files = try? FileManager.default.contentsOfDirectory(at: scope, includingPropertiesForKeys: [.fileSizeKey, .isRegularFileKey]) else { continue }
        var deleting = Set<String>()
        for file in files where file.pathExtension == "deleting" && OfflineManifest.digest(file.deletingPathExtension().lastPathComponent) {
          let key = file.deletingPathExtension().lastPathComponent
          deleting.insert(key)
          try? remove(scope: scope.lastPathComponent, key: key)
        }
        for file in files where ["transfer", "planning"].contains(file.pathExtension) && !deleting.contains(file.deletingPathExtension().lastPathComponent) {
          do {
            let values = try file.resourceValues(forKeys: [.fileSizeKey, .isRegularFileKey])
            let size = values.fileSize ?? 0
            guard values.isRegularFile == true, size > 0, size <= 2 * 1024 * 1024,
                  metadataBytes <= 32 * 1024 * 1024 - size, jobs.count + plans.count < 1000,
                  file.resolvingSymlinksInPath().standardizedFileURL == file.standardizedFileURL else { throw failure() }
            metadataBytes += size
            let data = try Data(contentsOf: file)
            if file.pathExtension == "planning" {
              let plan = try decodeOffline(DownloadPreparation.self, from: data)
              try plan.validate()
              guard file == planFile(plan), plans[plan.id] == nil else { throw failure() }
              plans[plan.id] = plan
            } else {
              let job = try decodeOffline(VerifiedDownload.self, from: data)
              try job.validate()
              guard file == journal(job), jobs[job.id] == nil else { throw failure() }
              jobs[job.id] = job
            }
          } catch {
            // Isolate one bad journal; never follow its contents to a different path.
            let quarantine = file.appendingPathExtension("invalid")
            if !FileManager.default.fileExists(atPath: quarantine.path) { try? FileManager.default.moveItem(at: file, to: quarantine) }
          }
        }
      }
      return (jobs, plans)
    }

    func save(_ job: VerifiedDownload) throws {
      try job.validate()
      try prepare(directory(job))
      try JSONEncoder().encode(job).write(to: journal(job), options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
    }

    func prepare(_ job: VerifiedDownload) throws {
      try job.validate()
      try prepare(directory(job))
    }

    func removePlan(_ plan: DownloadPreparation) throws {
      try plan.validate()
      try FileManager.default.removeItem(at: planFile(plan))
    }

    func availableBytes() throws -> Int64? {
      let size = try FileManager.default.attributesOfFileSystem(forPath: root.deletingLastPathComponent().path)[.systemFreeSize] as? NSNumber
      return size?.int64Value
    }

    func reset() throws {
      guard root.resolvingSymlinksInPath().standardizedFileURL == root.standardizedFileURL else { throw failure() }
      if FileManager.default.fileExists(atPath: root.path) { try FileManager.default.removeItem(at: root) }
    }

    private func failure() -> ClientError {
      VerifiedDownload.failure("The saved download could not be read. Remove it and try again.")
    }

    private func directory(_ job: VerifiedDownload) -> URL {
      root.appendingPathComponent(job.scope, isDirectory: true)
    }

    private func journal(_ job: VerifiedDownload) -> URL {
      directory(job).appendingPathComponent(job.key + ".transfer")
    }

    func media(_ job: VerifiedDownload) -> URL {
      directory(job).appendingPathComponent(job.key + ".media")
    }

    func staging(_ job: VerifiedDownload) -> URL {
      directory(job).appendingPathComponent(job.key + ".media.part")
    }

    private func prepare(_ url: URL) throws {
      guard url.resolvingSymlinksInPath().standardizedFileURL == url.standardizedFileURL else { throw failure() }
      try FileManager.default.createDirectory(at: url, withIntermediateDirectories: true,
                                              attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication])
      var url = url
      var values = URLResourceValues(); values.isExcludedFromBackup = true
      try url.setResourceValues(values)
    }

    func remove(scope: String, key: String) throws {
      guard OfflineManifest.digest(scope), OfflineManifest.digest(key) else { throw failure() }
      let folder = root.appendingPathComponent(scope)
      try prepare(folder)
      let marker = folder.appendingPathComponent(key + ".deleting")
      guard marker.resolvingSymlinksInPath().standardizedFileURL == marker.standardizedFileURL else { throw failure() }
      try Data().write(to: marker, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
      // Keep the durable intent until both payloads and journals are gone.
      for suffix in [".media", ".media.part", ".transfer", ".planning", ".transfer.invalid", ".planning.invalid"] {
        let file = folder.appendingPathComponent(key + suffix)
        if FileManager.default.fileExists(atPath: file.path) { try FileManager.default.removeItem(at: file) }
      }
      try FileManager.default.removeItem(at: marker)
    }

    private func planFile(_ plan: DownloadPreparation) -> URL {
      root.appendingPathComponent(plan.scope).appendingPathComponent(plan.key + ".planning")
    }

    func save(_ plan: DownloadPreparation) throws {
      try plan.validate(); try prepare(root.appendingPathComponent(plan.scope))
      try JSONEncoder().encode(plan).write(to: planFile(plan), options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
    }
  }
#endif
