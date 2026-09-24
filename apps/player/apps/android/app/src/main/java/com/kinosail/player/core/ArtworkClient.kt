package com.kinosail.player.core

import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Matrix
import androidx.exifinterface.media.ExifInterface
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
        return read(path, token, viewerId, MAX_BYTES)
    }

    fun photo(item: CatalogItem, token: String, viewerId: String): Bitmap {
        require(item.kind == "photo" && item.id.matches(Regex("[A-Za-z0-9_-]{1,128}")) &&
            item.stream == "/media/${item.id}" &&
            viewerId.matches(Regex("[A-Za-z0-9_-]{1,128}"))) { "Invalid photo request." }
        val data = read(item.stream, token, viewerId, MAX_PHOTO_BYTES)
        val decoded = decode(data, 4096, 250_000_000, 1)
        val exif = runCatching { ExifInterface(data.inputStream()) }.getOrNull() ?: return decoded
        val degrees = exif.rotationDegrees
        if (degrees == 0 && !exif.isFlipped) return decoded
        val matrix = Matrix().apply {
            postRotate(degrees.toFloat())
            if (exif.isFlipped) postScale(-1f, 1f)
        }
        return Bitmap.createBitmap(decoded, 0, 0, decoded.width, decoded.height, matrix, true)
    }

    fun comic(path: String, itemId: String, token: String, viewerId: String): Bitmap {
        require(ReaderApi.validComicPath(path, itemId) &&
            viewerId.matches(Regex("[A-Za-z0-9_-]{1,128}"))) { "Invalid comic page request." }
        return decode(read(path, token, viewerId, MAX_PHOTO_BYTES), 4096, 250_000_000, 1)
    }

    private fun read(path: String, token: String, viewerId: String, maximum: Int): ByteArray {
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
                boundedContentLength(connection, maximum.toLong())) { "The Server returned invalid artwork." }
            return connection.inputStream.use { stream ->
                val output = ByteArrayOutputStream()
                val buffer = ByteArray(8192)
                while (true) {
                    val count = stream.read(buffer)
                    if (count < 0) break
                    require(output.size() + count <= maximum) { "The artwork is too large." }
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
        return decode(bytes(path, token, viewerId), dimension)
    }

    private fun decode(data: ByteArray, dimension: Int, maxPixels: Long = 80_000_000,
                       oversample: Int = 2): Bitmap {
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeByteArray(data, 0, data.size, bounds)
        require(bounds.outWidth in 1..32_768 && bounds.outHeight in 1..32_768 &&
            bounds.outWidth.toLong() * bounds.outHeight <= maxPixels) { "The artwork dimensions are invalid." }
        var sample = 1
        while (maxOf(bounds.outWidth, bounds.outHeight) / sample > dimension * oversample) sample *= 2
        val options = BitmapFactory.Options().apply { inSampleSize = sample }
        return requireNotNull(BitmapFactory.decodeByteArray(data, 0, data.size, options)) {
            "The artwork could not be decoded."
        }
    }

    companion object {
        private const val MAX_BYTES = 16 * 1024 * 1024
        private const val MAX_PHOTO_BYTES = 64 * 1024 * 1024
    }
}
