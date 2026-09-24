package com.kinosail.player.core

import java.net.URI

class MediaUriPolicy(private val server: ServerAddress, private val itemId: String) {
    init { require(itemId.matches(Regex("[A-Za-z0-9_-]{1,128}"))) { "Invalid media item." } }

    fun requireAllowed(raw: String): String {
        require(raw.length <= 16_384 && raw.none(Char::isISOControl)) { INVALID_MEDIA }
        val uri = try { URI(raw) } catch (_: Exception) { throw IllegalArgumentException(INVALID_MEDIA) }
        val origin = server.url.toURI()
        val port = if (uri.port == -1) defaultPort(uri.scheme) else uri.port
        val serverPort = if (origin.port == -1) defaultPort(origin.scheme) else origin.port
        require(uri.scheme?.equals(origin.scheme, true) == true &&
            uri.host?.equals(origin.host, true) == true && port == serverPort &&
            uri.userInfo == null && uri.rawQuery == null && uri.rawFragment == null) { INVALID_MEDIA }
        val path = uri.rawPath ?: throw IllegalArgumentException(INVALID_MEDIA)
        require(path == "/media/$itemId" || path.startsWith("/hls/$itemId/") && validHls(path)) { INVALID_MEDIA }
        return uri.toString()
    }

    private fun validHls(path: String): Boolean {
        val rest = path.removePrefix("/hls/$itemId/")
        val segments = rest.split('/')
        return segments.size in 1..16 && segments.all { segment ->
            segment.isNotEmpty() && segment != "." && segment != ".." &&
                segment.length <= 8192 && segment.all { it.isLetterOrDigit() && it.code < 128 || it in "_.-" }
        }
    }

    companion object {
        private const val INVALID_MEDIA = "The Server returned an invalid media address."
        private fun defaultPort(scheme: String?): Int = when (scheme?.lowercase()) {
            "https" -> 443
            "http" -> 80
            else -> -1
        }
    }
}
