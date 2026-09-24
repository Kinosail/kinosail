package com.kinosail.player.core

import android.content.Context
import android.media.MediaCodecList
import android.os.Build
import android.view.Display
import android.view.WindowManager

data class PlaybackCapabilities(
    val video: List<String>,
    val audio: List<String>,
    val hdr: List<String>,
    val maxAudioChannels: Int,
) {
    init {
        require(video.isNotEmpty() && video.size <= 4 && video.distinct() == video &&
            video.all { it in VIDEO } && audio.isNotEmpty() && audio.size <= 6 &&
            audio.distinct() == audio && audio.all { it in AUDIO } && hdr.contains("sdr") &&
            hdr.size <= 3 && hdr.distinct() == hdr && hdr.all { it in HDR } && maxAudioChannels in 1..8) {
            "Invalid playback capabilities."
        }
    }

    val query: String get() = "videoCodecs=${video.joinToString(",")}&audioCodecs=${audio.joinToString(",")}" +
        "&hdrFormats=${hdr.joinToString(",")}&maxAudioChannels=$maxAudioChannels"

    companion object {
        private val VIDEO = setOf("h264", "hevc", "av1", "vp9")
        private val AUDIO = setOf("aac", "mp3", "opus", "vorbis", "ac3", "eac3")
        private val HDR = setOf("sdr", "hdr10", "hlg")

        @Suppress("DEPRECATION")
        fun detect(context: Context): PlaybackCapabilities {
            val types = MediaCodecList(MediaCodecList.REGULAR_CODECS).codecInfos
                .filterNot { it.isEncoder }.flatMap { it.supportedTypes.asList() }.toSet()
            val video = linkedMapOf("h264" to "video/avc", "hevc" to "video/hevc",
                "av1" to "video/av01", "vp9" to "video/x-vnd.on2.vp9")
                .filterValues(types::contains).keys.toList().ifEmpty { listOf("h264") }
            val audio = linkedMapOf("aac" to "audio/mp4a-latm", "mp3" to "audio/mpeg",
                "opus" to "audio/opus", "vorbis" to "audio/vorbis", "ac3" to "audio/ac3",
                "eac3" to "audio/eac3").filterValues(types::contains).keys.toList()
                .ifEmpty { listOf("aac", "mp3") }
            val hdr = mutableListOf("sdr")
            if (Build.VERSION.SDK_INT >= 24 && video.any { it == "hevc" || it == "av1" }) {
                val display = (context.getSystemService(Context.WINDOW_SERVICE) as WindowManager).defaultDisplay
                val supported = display.hdrCapabilities?.supportedHdrTypes?.toSet().orEmpty()
                if (Display.HdrCapabilities.HDR_TYPE_HDR10 in supported) hdr += "hdr10"
                if (Display.HdrCapabilities.HDR_TYPE_HLG in supported) hdr += "hlg"
            }
            return PlaybackCapabilities(video, audio, hdr, 2)
        }
    }
}
