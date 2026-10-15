package androidx.media3.exoplayer.source

import androidx.media3.common.Format
import androidx.media3.extractor.text.SubtitleExtractor

@androidx.annotation.OptIn(markerClass = [androidx.media3.common.util.UnstableApi::class])
internal fun ProgressiveMediaSource.Factory.kinosailLazySubtitleTrack(format: Format): ProgressiveMediaSource.Factory =
    enableLazyLoadingWithSingleTrack(SubtitleExtractor.TRACK_ID, format)
