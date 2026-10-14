package com.kinosail.player.core

import androidx.media3.common.Format
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.datasource.DataSource
import androidx.media3.exoplayer.drm.DrmSessionManagerProvider
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.exoplayer.source.MediaSource
import androidx.media3.exoplayer.source.MergingMediaSource
import androidx.media3.exoplayer.source.ProgressiveMediaSource
import androidx.media3.exoplayer.source.kinosailLazySubtitleTrack
import androidx.media3.exoplayer.upstream.LoadErrorHandlingPolicy
import androidx.media3.extractor.ExtractorsFactory
import androidx.media3.extractor.text.DefaultSubtitleParserFactory
import androidx.media3.extractor.text.SubtitleExtractor

@androidx.annotation.OptIn(markerClass = [androidx.media3.common.util.UnstableApi::class])
internal class PlaybackMediaSourceFactory(private val data: DataSource.Factory) : MediaSource.Factory {
    private val primary = DefaultMediaSourceFactory(data)
    private var errorPolicy: LoadErrorHandlingPolicy? = null

    override fun getSupportedTypes(): IntArray = primary.supportedTypes

    override fun setDrmSessionManagerProvider(provider: DrmSessionManagerProvider): MediaSource.Factory {
        primary.setDrmSessionManagerProvider(provider)
        return this
    }

    override fun setLoadErrorHandlingPolicy(policy: LoadErrorHandlingPolicy): MediaSource.Factory {
        primary.setLoadErrorHandlingPolicy(policy)
        errorPolicy = policy
        return this
    }

    override fun createMediaSource(item: MediaItem): MediaSource {
        val subtitles = item.localConfiguration?.subtitleConfigurations.orEmpty()
        if (subtitles.isEmpty() || subtitles.any { it.mimeType != MimeTypes.TEXT_VTT })
            return primary.createMediaSource(item)
        val sources = mutableListOf(primary.createMediaSource(item.buildUpon()
            .setSubtitleConfigurations(emptyList()).build()))
        val parsers = DefaultSubtitleParserFactory()
        for (subtitle in subtitles) {
            val format = Format.Builder().setSampleMimeType(subtitle.mimeType)
                .setId(subtitle.id).setLanguage(subtitle.language).setLabel(subtitle.label)
                .setSelectionFlags(subtitle.selectionFlags).setRoleFlags(subtitle.roleFlags).build()
            // Passing the original format lets the extractor publish its measured cue
            // end through the seek map. A null format loses it in Media3 1.11.1, so
            // re-selection beyond the last cue START incorrectly ends the text stream.
            val extractors = ExtractorsFactory { arrayOf(SubtitleExtractor(parsers.create(format), format)) }
            val output = format.buildUpon().setSampleMimeType(MimeTypes.APPLICATION_MEDIA3_CUES)
                .setCodecs(format.sampleMimeType)
                .setCueReplacementBehavior(parsers.getCueReplacementBehavior(format)).build()
            val factory = ProgressiveMediaSource.Factory(data, extractors).kinosailLazySubtitleTrack(output)
            errorPolicy?.let(factory::setLoadErrorHandlingPolicy)
            sources.add(factory.createMediaSource(MediaItem.fromUri(subtitle.uri)))
        }
        // Preserve the primary duration and timeline; captions may finish before the movie.
        return MergingMediaSource(*sources.toTypedArray())
    }
}
