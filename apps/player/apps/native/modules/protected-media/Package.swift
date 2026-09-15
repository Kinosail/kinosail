// swift-tools-version: 5.9
import PackageDescription

let package = Package(
  name: "DownloadRecovery",
  platforms: [.macOS(.v12), .iOS(.v16)],
  targets: [
    .target(name: "DownloadRecovery", path: "ios", exclude: [
      "BackgroundDownloads.swift", "LocalVideoView.swift", "ProtectedMediaModule.swift",
      "ProtectedReaderModule.swift", "PlaybackCapabilitiesModule.swift", "ProtectedMedia.podspec", "KinosailAudioProcessor.h", "KinosailAudioProcessor.m"
    ], sources: ["DownloadRecovery.swift", "MediaTransferSession.swift"]),
    .testTarget(name: "DownloadRecoveryTests", dependencies: ["DownloadRecovery"], path: "tests")
  ]
)
