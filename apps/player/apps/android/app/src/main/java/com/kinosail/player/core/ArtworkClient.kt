package com.kinosail.player.core

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL

class ArtworkClient(
    private val server: ServerAddress,
    private val open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection },
) {
    fun bytes(path: String, token: String, viewerId: String): ByteArray {
        require(path.matches(CatalogApi.ARTWORK) &&
            viewerId.matches(Regex("[A-Za-z0-9_-]{1,128}"))) { "Invalid artwork request." }
        ServerApi.checkedCredential(token, 512)
        val connection = open(URL(server.url, path))
        try {
            connection.requestMethod = "GET"
            connection.instanceFollowRedirects = false
            connection.connectTimeout = 10_000
            connection.readTimeout = 20_000
            connection.useCaches = false
            connection.setRequestProperty("Accept", "image/avif,image/webp,image/png,image/jpeg")
            connection.setRequestProperty("Authorization", "Bearer $token")
            connection.setRequestProperty("X-Kinosail-Viewer-Profile", viewerId)
            require(connection.responseCode == 200 &&
                connection.contentType?.substringBefore(';')?.trim()?.lowercase()?.startsWith("image/") == true &&
                connection.contentLengthLong <= MAX_BYTES) { "The Server returned invalid artwork." }
            return connection.inputStream.use { stream ->
                val output = ByteArrayOutputStream()
                val buffer = ByteArray(8192)
                while (true) {
                    val count = stream.read(buffer)
                    if (count < 0) break
                    require(output.size() + count <= MAX_BYTES) { "The artwork is too large." }
                    output.write(buffer, 0, count)
                }
                output.toByteArray()
            }
        } finally {
            connection.disconnect()
        }
    }

    fun bitmap(path: String, token: String, viewerId: String, dimension: Int): Bitmap {
        require(dimension == 400 || dimension == 800) { "Invalid artwork size." }
        val data = bytes(path, token, viewerId)
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeByteArray(data, 0, data.size, bounds)
        require(bounds.outWidth in 1..32_768 && bounds.outHeight in 1..32_768 &&
            bounds.outWidth.toLong() * bounds.outHeight <= 80_000_000) { "The artwork dimensions are invalid." }
        var sample = 1
        while (maxOf(bounds.outWidth, bounds.outHeight) / sample > dimension * 2) sample *= 2
        val options = BitmapFactory.Options().apply { inSampleSize = sample }
        return requireNotNull(BitmapFactory.decodeByteArray(data, 0, data.size, options)) {
            "The artwork could not be decoded."
        }
    }

    companion object { private const val MAX_BYTES = 16 * 1024 * 1024 }
}
