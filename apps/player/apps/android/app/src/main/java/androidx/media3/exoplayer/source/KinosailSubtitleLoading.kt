package androidx.media3.exoplayer.source

import androidx.media3.common.Format
import androidx.media3.extractor.text.SubtitleExtractor

// Media3 1.11.1 exposes lazy single-track preparation only inside this package.
// Keep this compile-checked bridge until DefaultMediaSourceFactory preserves the
// SubtitleExtractor seek map/duration. Do not replace it with eager sidecar loads.
@androidx.annotation.OptIn(markerClass = [androidx.media3.common.util.UnstableApi::class])
internal fun ProgressiveMediaSource.Factory.kinosailLazySubtitleTrack(format: Format): ProgressiveMediaSource.Factory =
    enableLazyLoadingWithSingleTrack(SubtitleExtractor.TRACK_ID, format)
