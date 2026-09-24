package com.kinosail.player.core

import android.graphics.Bitmap
import android.graphics.Color
import android.graphics.pdf.PdfRenderer
import android.os.ParcelFileDescriptor
import java.io.DataInputStream
import java.io.File
import java.net.HttpURLConnection
import java.net.URI
import java.net.URL
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.intOrNull

data class ReaderBook(val id: String, val title: String, val filePath: String)
data class ComicBook(val id: String, val title: String, val pages: List<String>)
data class ReaderPosition(val page: Int, val total: Int, val offset: Double)

class ReaderApi(
    private val server: ServerAddress,
    private val open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection },
) {
    private val api = ServerApi(server, open)

    fun format(itemId: String, token: String, viewerId: String): String {
        checkInputs(itemId, token, viewerId)
        val fields = api.reader(itemId, token, viewerId).fields(setOf("id", "title", "type", "pages"))
        require(fields.string("id", 128) == itemId) { INVALID_RESPONSE }
        fields.string("title", 512)
        val type = fields.string("type", 16)
        require(type in setOf("pdf", "epub", "comic")) { INVALID_RESPONSE }
        val pages = fields["pages"] as? JsonArray
        require(pages != null && pages.size in 1..10_000) { INVALID_RESPONSE }
        return type
    }

    fun book(itemId: String, token: String, viewerId: String): ReaderBook {
        checkInputs(itemId, token, viewerId)
        val fields = api.reader(itemId, token, viewerId).fields(setOf("id", "title", "type", "pages"))
        require(fields.string("id", 128) == itemId) { INVALID_RESPONSE }
        val type = fields.string("type", 16)
        require(type in setOf("pdf", "epub", "comic")) { INVALID_RESPONSE }
        require(type == "pdf") { "This book format is not available on Android yet." }
        val pages = fields["pages"] as? JsonArray
        require(pages != null && pages.size == 1) { INVALID_RESPONSE }
        val page = pages[0].fields(setOf("number", "title", "url"))
        val path = "/read/$itemId/file"
        page.string("title", 512, empty = true)
        require(page.integer("number") == 1 && page.string("url", 2048) == path) { INVALID_RESPONSE }
        return ReaderBook(itemId, fields.string("title", 512), path)
    }

    fun comic(itemId: String, token: String, viewerId: String): ComicBook {
        checkInputs(itemId, token, viewerId)
        val fields = api.reader(itemId, token, viewerId).fields(setOf("id", "title", "type", "pages"))
        require(fields.string("id", 128) == itemId && fields.string("type", 16) == "comic") {
            INVALID_RESPONSE
        }
        val pages = fields["pages"] as? JsonArray
        require(pages != null && pages.size in 1..10_000) { INVALID_RESPONSE }
        val paths = pages.mapIndexed { index, raw ->
            val page = raw.fields(setOf("number", "title", "url"))
            page.string("title", 512, empty = true)
            val path = page.string("url", 2048)
            require(page.integer("number") == index + 1 && validComicPath(path, itemId)) { INVALID_RESPONSE }
            path
        }
        require(paths.toSet().size == paths.size) { INVALID_RESPONSE }
        return ComicBook(itemId, fields.string("title", 512), paths)
    }

    fun position(itemId: String, token: String, viewerId: String, expectedTotal: Int = 1): ReaderPosition {
        checkInputs(itemId, token, viewerId)
        require(expectedTotal in 1..10_000) { "Invalid reader request." }
        return parsePosition(api.readerPosition(itemId, token, viewerId), expectedTotal)
    }

    fun save(itemId: String, token: String, viewerId: String, offset: Double,
             page: Int = 1, expectedTotal: Int = 1): ReaderPosition {
        checkInputs(itemId, token, viewerId)
        require(offset.isFinite() && offset in 0.0..1.0 && expectedTotal in 1..10_000 &&
            page in 1..expectedTotal) { "Invalid reading position." }
        val result = parsePosition(api.reader(itemId, token, viewerId, offset, page), expectedTotal)
        require(result.page == page && result.offset == offset) { INVALID_RESPONSE }
        return result
    }

    fun download(book: ReaderBook, token: String, viewerId: String, cacheDir: File): File {
        checkInputs(book.id, token, viewerId)
        require(book.filePath == "/read/${book.id}/file") { "Invalid reader file." }
        val connection = open(URL(server.url, book.filePath))
        var file: File? = null
        try {
            connection.requestMethod = "GET"
            connection.instanceFollowRedirects = false
            connection.connectTimeout = 10_000
            connection.readTimeout = 30_000
            connection.useCaches = false
            connection.setRequestProperty("Accept", "application/pdf")
            connection.setRequestProperty("Authorization", "Bearer $token")
            connection.setRequestProperty("X-Kinosail-Viewer-Profile", viewerId)
            require(connection.responseCode == 200 &&
                connection.contentType?.substringBefore(';')?.trim()?.lowercase() == "application/pdf" &&
                boundedContentLength(connection, MAX_PDF_BYTES)) { INVALID_RESPONSE }
            val temporary = File.createTempFile("kinosail-reader-", ".pdf", cacheDir)
            file = temporary
            connection.inputStream.use { input ->
                temporary.outputStream().use { output ->
                    val buffer = ByteArray(8192)
                    var count = 0L
                    while (true) {
                        val read = input.read(buffer)
                        if (read < 0) break
                        count += read
                        require(count <= MAX_PDF_BYTES) { "The PDF is too large." }
                        output.write(buffer, 0, read)
                    }
                    require(count >= 5) { INVALID_RESPONSE }
                }
            }
            require(temporary.inputStream().use { input ->
                val header = ByteArray(5)
                DataInputStream(input).readFully(header)
                header.contentEquals("%PDF-".toByteArray())
            }) {
                INVALID_RESPONSE
            }
            return temporary
        } catch (failure: Exception) {
            file?.delete()
            throw failure
        } finally {
            connection.disconnect()
        }
    }

    private fun parsePosition(raw: JsonElement, expectedTotal: Int): ReaderPosition {
        val fields = raw.fields(setOf("page", "total", "offset"))
        val page = fields.integer("page")
        val value = (fields["offset"] as? JsonPrimitive)?.takeUnless { it.isString }?.doubleOrNull
        require(page != null && page in 1..expectedTotal && fields.integer("total") == expectedTotal &&
            value != null && value.isFinite() && value in 0.0..1.0) { INVALID_RESPONSE }
        return ReaderPosition(page, expectedTotal, value)
    }

    private fun checkInputs(itemId: String, token: String, viewerId: String) {
        require(itemId.matches(ID) && viewerId.matches(ID)) { "Invalid reader request." }
        ServerApi.checkedCredential(token, 512)
    }

    companion object {
        private const val INVALID_RESPONSE = "The Server returned an invalid reader response."
        private const val MAX_PDF_BYTES = 128L * 1024 * 1024
        private val ID = Regex("[A-Za-z0-9_-]{1,128}")

        internal fun validComicPath(path: String, itemId: String): Boolean {
            if (!itemId.matches(ID) || path.toByteArray(Charsets.UTF_8).size > 2048 ||
                !path.startsWith("/read/$itemId/asset/") || path.endsWith('/') ||
                path.any { it.isWhitespace() || it.isISOControl() }) return false
            val uri = runCatching { URI(path) }.getOrNull() ?: return false
            if (uri.scheme != null || uri.rawAuthority != null || uri.rawQuery != null || uri.rawFragment != null ||
                uri.rawPath != path) return false
            val prefix = "/read/$itemId/asset/"
            val raw = path.removePrefix(prefix).split('/')
            val decoded = uri.path.removePrefix(prefix).split('/')
            return raw.size == decoded.size && decoded.all { it.isNotEmpty() && it != "." && it != ".." &&
                '\\' !in it && it.none(Char::isISOControl) }
        }

        private fun JsonElement.fields(keys: Set<String>): JsonObject {
            val value = this as? JsonObject
            require(value != null && value.keys == keys) { INVALID_RESPONSE }
            return value
        }

        private fun JsonObject.string(key: String, max: Int, empty: Boolean = false): String {
            val value = this[key] as? JsonPrimitive
            require(value != null && value.isString && (empty || value.content.isNotEmpty()) &&
                value.content.toByteArray(Charsets.UTF_8).size <= max &&
                value.content.none(Char::isISOControl)) { INVALID_RESPONSE }
            return value.content
        }

        private fun JsonObject.integer(key: String): Int? =
            (this[key] as? JsonPrimitive)?.takeUnless { it.isString }?.intOrNull
    }
}

class PdfDocument(private val file: File) : AutoCloseable {
    private val renderer: PdfRenderer = ParcelFileDescriptor.open(file, ParcelFileDescriptor.MODE_READ_ONLY).let { fd ->
        try { PdfRenderer(fd) } catch (failure: Exception) { fd.close(); throw failure }
    }
    val pageCount: Int = renderer.pageCount.also {
        if (it !in 1..10_000) { renderer.close(); throw IllegalArgumentException("The PDF has no readable pages.") }
    }

    @Synchronized fun render(page: Int): Bitmap {
        require(page in 0 until pageCount) { "Invalid PDF page." }
        renderer.openPage(page).use { source ->
            require(source.width in 1..100_000 && source.height in 1..100_000) { "Invalid PDF page size." }
            val scale = minOf(2048f / source.width, 2048f / source.height, 2f)
            val bitmap = Bitmap.createBitmap((source.width * scale).toInt().coerceAtLeast(1),
                (source.height * scale).toInt().coerceAtLeast(1), Bitmap.Config.ARGB_8888)
            bitmap.eraseColor(Color.WHITE)
            source.render(bitmap, null, null, PdfRenderer.Page.RENDER_MODE_FOR_DISPLAY)
            return bitmap
        }
    }

    @Synchronized override fun close() {
        renderer.close()
        file.delete()
    }
}
