#if os(iOS)
  import Foundation
  import UIKit

  extension VerifiedDownloads {
    func urlSession(_: URLSession, downloadTask: URLSessionDownloadTask, didWriteData _: Int64,
                    totalBytesWritten: Int64, totalBytesExpectedToWrite: Int64)
    {
      if planTasks[downloadTask.taskIdentifier] != nil {
        if totalBytesWritten > 2 * 1024 * 1024 || totalBytesExpectedToWrite > 2 * 1024 * 1024 {
          downloadTask.cancel()
        }; return
      }
      guard let extent = tasks[downloadTask.taskIdentifier], let job = jobs[extent.id] else { return }
      let length = min(job.manifest.size, Int64(extent.first + extent.count) * job.manifest.chunkSize) - Int64(extent.first) * job.manifest.chunkSize
      if totalBytesWritten > length || totalBytesExpectedToWrite > length {
        downloadTask.cancel(); return
      }
      progress[downloadTask.taskIdentifier] = max(0, totalBytesWritten)
    }

    func urlSession(_: URLSession, downloadTask: URLSessionDownloadTask, didFinishDownloadingTo location: URL) {
      if let (_, id) = planTasks[downloadTask.taskIdentifier] {
        receivedPlan(id, task: downloadTask, location: location); return
      }
      guard let extent = tasks[downloadTask.taskIdentifier], var job = jobs[extent.id] else { return }
      do {
        let start = Int64(extent.first) * job.manifest.chunkSize
        let end = min(job.manifest.size, Int64(extent.first + extent.count) * job.manifest.chunkSize) - 1
        guard let response = downloadTask.response as? HTTPURLResponse else { throw failure() }
        if let delay = DownloadRecovery.delay(error: nil, response: response, attempt: job.retries) {
          job.retries += 1; job.retryAt = Date(timeIntervalSinceNow: delay); job.status = "waiting"; job.error = "The Server is busy. Retrying automatically."
          try persist(job); return
        }
        guard response.statusCode == 206, response.url?.absoluteString == job.uri,
              response.value(forHTTPHeaderField: "ETag") == "\"\(job.manifest.sha256)\"",
              response.value(forHTTPHeaderField: "Content-Range") == "bytes \(start)-\(end)/\(job.manifest.size)",
              response.value(forHTTPHeaderField: "Content-Length") == String(end - start + 1),
              try location.resourceValues(forKeys: [.fileSizeKey]).fileSize == Int(end - start + 1)
        else { throw failure() }
        let input = try FileHandle(forReadingFrom: location); defer { try? input.close() }
        let output = try FileHandle(forWritingTo: store.staging(job)); defer { try? output.close() }
        var corrupt = false
        for index in extent.first ..< (extent.first + extent.count) {
          let block = try input.read(upToCount: Int(job.manifest.length(index))) ?? Data()
          guard block.count == Int(job.manifest.length(index)), VerifiedDownload.hash(block) == job.manifest.chunks[index] else { corrupt = true; continue }
          try output.seek(toOffset: UInt64(Int64(index) * job.manifest.chunkSize))
          try output.write(contentsOf: block)
          try output.synchronize()
          job.verified[index] = true
          try persist(job) // Data is durable before its verified bit.
        }
        if corrupt {
          job.retries += 1; if job.retries > 5 {
            job.retries = 5; job.status = "paused"; job.error = "Part of the download is missing or damaged. Resume to download that part again."
          }
        } else {
          job.retries = 0; job.retryAt = nil; job.error = ""
        }
        try persist(job)
      } catch {
        job.status = "paused"; job.error = "Could not verify the download. Resume to try again; the parts already checked are kept."
        do { try persist(job) } catch { storageError = true }
      }
    }

    func urlSession(_: URLSession, task: URLSessionTask, didCompleteWithError error: Error?) {
      if let (_, id) = planTasks.removeValue(forKey: task.taskIdentifier) {
        if let error, var plan = plans[id] {
          if let delay = DownloadRecovery.delay(error: error as NSError, response: task.response as? HTTPURLResponse, attempt: plan.retries ?? 0) {
            plan.retries = (plan.retries ?? 0) + 1
            plan.retryAt = Date(timeIntervalSinceNow: max(15, delay))
          } else {
            plan.status = "paused"; plan.error = "Preparation interrupted. Resume to try again."
          }
          try? savePlan(plan)
        }
        pump(); return
      }
      guard let extent = tasks.removeValue(forKey: task.taskIdentifier), var job = jobs[extent.id] else { return }
      progress.removeValue(forKey: task.taskIdentifier)
      if let error, job.status != "paused" {
        if let delay = DownloadRecovery.delay(error: error as NSError, response: task.response as? HTTPURLResponse, attempt: job.retries) {
          job.retryAt = Date(timeIntervalSinceNow: delay); job.retries += 1; job.status = "waiting"; job.error = "Connection interrupted. Retrying automatically."
        } else {
          job.status = "paused"; job.error = "Download interrupted. Resume to continue."
        }
        do { try persist(job) } catch { storageError = true }
      }
      pump()
    }

    func finish(_ input: VerifiedDownload) {
      guard !checking.contains(input.id) else { return }
      var job = input
      job.status = "verifying"
      do { try persist(job) } catch { storageError = true; return }
      let path = FileManager.default.fileExists(atPath: store.staging(job).path) ? store.staging(job) : store.media(job)
      let verification = DownloadVerification()
      checking.insert(job.id); verifications[job.id] = verification
      let candidate = job
      let generation = probeGenerations[job.id, default: 0]
      verificationQueue.async {
        let result = Result { try verification.verify(candidate, at: path) }
        self.queue.async {
          guard self.verifications[input.id] === verification else { return }
          switch result {
          case .failure:
            self.completeVerification(input, generation: generation, path: path, playable: false, stamp: nil)
          case .success(let (checked, stamp)):
            guard checked.status == "verifying" else {
              self.verifications.removeValue(forKey: input.id); self.checking.remove(input.id)
              if self.probeGenerations[input.id, default: 0] == generation, self.jobs[input.id]?.status == "verifying" {
                do { try self.persist(checked) } catch { self.storageError = true }
              }
              self.pump(); self.finishBackgroundEvents(); return
            }
            OfflineProbe.check(path, video: checked.kind == "video") { playable in
              self.queue.async {
                guard self.verifications[input.id] === verification else { return }
                self.completeVerification(checked, generation: generation, path: path, playable: playable, stamp: stamp)
              }
            }
          }
        }
      }
    }

    private func completeVerification(_ input: VerifiedDownload, generation: Int, path: URL, playable: Bool, stamp: DownloadFileStamp?) {
      checking.remove(input.id); verifications.removeValue(forKey: input.id)
      defer { pump(); finishBackgroundEvents() }
      guard probeGenerations[input.id, default: 0] == generation, authorization?.scope == input.scope,
            var current = jobs[input.id], current.status == "verifying" else { return }
      do {
        guard let stamp, playable, stamp == (try DownloadFileStamp(path)) else { throw failure() }
        if path != store.media(current) { try FileManager.default.moveItem(at: path, to: store.media(current)) }
        current.status = "complete"; current.error = ""
        let finalStamp = try DownloadFileStamp(store.media(current))
        try persist(current)
        verifiedFiles[current.id] = finalStamp
      } catch {
        current.status = "paused"
        current.error = stamp != nil ? "The file downloaded correctly, but this device could not play it. Download a compatible version." : "The download check stopped. Resume to check again."
        do { try persist(current) } catch { storageError = true }
      }
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, willPerformHTTPRedirection response: HTTPURLResponse,
                    newRequest request: URLRequest, completionHandler: @escaping @Sendable (URLRequest?) -> Void) { completionHandler(nil) }

  }
#endif
