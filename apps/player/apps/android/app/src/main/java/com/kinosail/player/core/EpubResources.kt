package com.kinosail.player.core

import android.util.Log
import android.webkit.WebResourceResponse
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URI
import java.net.URL
import java.nio.ByteBuffer
import java.nio.charset.CodingErrorAction
import java.util.UUID

/** One chapter owns its request and byte budget. WebView never owns credentials or network access. */
internal class EpubResources(private val itemId: String, private val saved: SavedSession,
    private val viewer: Viewer, private val fontSize: Int, private val light: Boolean) {
    private var requests = 0
    private var bytesUsed = 0
    private var reportedBlocked = false

    @Synchronized fun load(raw: String): WebResourceResponse {
        val path = path(raw, itemId)
        if (path == null) {
            if (!reportedBlocked) Log.w("KinosailReader", "Resource rejected operation=reader-resource kind=outside_book outcome=blocked")
            reportedBlocked = true
            return blocked()
        }
        val requestId = UUID.randomUUID().toString()
        var connection: HttpURLConnection? = null
        var status = 0
        try {
            require(++requests <= 512 && bytesUsed < MAX_TOTAL)
            require(saved.viewer?.let { it.serverId == viewer.serverId && it.id == viewer.id } == true)
            connection = URL(saved.server.url, path).openConnection() as HttpURLConnection
            connection.instanceFollowRedirects = false
            connection.connectTimeout = 10_000
            connection.readTimeout = 20_000
            connection.useCaches = false
            connection.setRequestProperty("Authorization", "Bearer ${ServerApi.checkedCredential(saved.token, 512)}")
            connection.setRequestProperty("X-Kinosail-Viewer-Profile", viewer.id)
            connection.setRequestProperty("X-Request-ID", requestId)
            status = connection.responseCode
            require(status == 200)
            val mime = connection.contentType?.substringBefore(';')?.trim()?.lowercase() ?: ""
            require(mime in TYPES || mime.startsWith("image/") || mime.startsWith("font/"))
            val html = mime in setOf("text/html", "application/xhtml+xml")
            val maximum = minOf(if (html) 8 * 1024 * 1024 else 32 * 1024 * 1024, MAX_TOTAL - bytesUsed)
            require(boundedContentLength(connection, maximum.toLong()))
            val content = connection.inputStream.use { input ->
                val output = ByteArrayOutputStream()
                val buffer = ByteArray(8192)
                while (true) {
                    val count = input.read(buffer)
                    if (count < 0) break
                    bytesUsed += count
                    require(output.size() + count <= maximum && bytesUsed <= MAX_TOTAL)
                    output.write(buffer, 0, count)
                }
                output.toByteArray()
            }
            val body = if (html) {
                val text = Charsets.UTF_8.newDecoder().onMalformedInput(CodingErrorAction.REPORT).decode(ByteBuffer.wrap(content))
                val colors = if (light) "#f4f8ef" to "#162011" else "#0b0d0b" to "#f6f8f2"
                val style = "html,body{background:${colors.first}!important;color:${colors.second}!important;font-size:${fontSize}px!important;line-height:1.65!important;}body{padding:16px!important;max-width:68ch!important;margin:auto!important;}img,svg{max-width:100%!important;height:auto!important;}a{color:inherit;}"
                ("<meta http-equiv=\"Content-Security-Policy\" content=\"$POLICY\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><style>$style</style>" + text).toByteArray()
            } else content
            return WebResourceResponse(if (html) "text/html" else mime, "utf-8", 200, "OK",
                mapOf("Content-Security-Policy" to POLICY, "Cache-Control" to "no-store", "X-Content-Type-Options" to "nosniff"), body.inputStream())
        } catch (failure: Exception) {
            Log.w("KinosailReader", "Resource failed request_id=$requestId operation=reader-resource status=$status kind=${if (status != 0 && status != 200) "http" else "invalid_or_unavailable"} outcome=blocked")
            return WebResourceResponse("text/plain", "utf-8", 502, "Unavailable", emptyMap(), ByteArray(0).inputStream())
        } finally { connection?.disconnect() }
    }

    companion object {
        const val ORIGIN = "https://kinosail-reader.invalid"
        private const val MAX_TOTAL = 64 * 1024 * 1024
        private const val POLICY = "default-src 'none'; script-src 'none'; img-src https://kinosail-reader.invalid; style-src https://kinosail-reader.invalid 'unsafe-inline'; font-src https://kinosail-reader.invalid; base-uri 'none'; form-action 'none'; frame-src 'none'; object-src 'none'; connect-src 'none'"
        private val TYPES = setOf("text/html", "application/xhtml+xml", "text/css", "text/plain", "application/font-woff", "application/vnd.ms-opentype")
        fun path(raw: String, itemId: String): String? = runCatching {
            val uri = URI(raw)
            uri.rawPath.takeIf { uri.scheme == "https" && uri.rawAuthority == "kinosail-reader.invalid" &&
                uri.rawQuery == null && ReaderApi.validComicPath(it, itemId) }
        }.getOrNull()
        fun blocked() = WebResourceResponse("text/plain", "utf-8", 403, "Blocked", emptyMap(), ByteArray(0).inputStream())
    }
}
