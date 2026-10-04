# Q09 Subtitles Player asset delivery repair

At exact candidate `6e4390e854aee125069c9199e555d3616b0fede7`, normal PR 475 Subtitles Go job `111510941466` failed the existing public `TestTranscodedDownloadBecomesIntegrityCheckedReadyOfflineFile` assertion at `packages/servertest/downloads_offline.go:72`. The offline page and fixture expected `/static/downloads.js?v=14-htmx4`; the rendered Subtitles Player page still served version 13. This is a task-caused missed include, not an FFmpeg fixture failure.

The one-line production repair changes the Subtitles Player replacement include from version 13 to 14. The existing public assertion and all device-transfer behavior remain unchanged. Normal exact-head Subtitles Go validation and the full focused 14-case hosted proof must pass after integration. The native paused-seek owner has reported no asset include edits at its current checkpoint.

Private job output: 1,710,097 bytes; SHA-256 `b98b5176d0bea459a6250e881969c97d76b69eb3dc3d06c784cd00d02f973a62`. Only the fixed source location and fixed served/expected asset versions are included here. No private HTML or raw process log is published.
