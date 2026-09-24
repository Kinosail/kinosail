package com.kinosail.player.core

import android.graphics.Bitmap
import androidx.exifinterface.media.ExifInterface
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner

@RunWith(RobolectricTestRunner::class)
class PhotoClientTest {
    private val server = ServerAddress("https://example.com")
    private val item = CatalogItem("photo-1", "photo", "Harbor", "", "", "",
        stream = "/media/photo-1")

    @Test fun fetchesAndDecodesOnlyTheViewersOriginalPhoto() {
        val encoded = ByteArrayOutputStream().also {
            Bitmap.createBitmap(2, 2, Bitmap.Config.ARGB_8888).compress(Bitmap.CompressFormat.PNG, 100, it)
        }.toByteArray()
        val response = PhotoResponse(200, encoded)
        val image = ArtworkClient(server) { url -> response.also { it.requestedURL = url } }
            .photo(item, "token", "alex")
        assertEquals(2, image.width)
        assertEquals("https://example.com/media/photo-1", response.requestedURL.toString())
        assertEquals("Bearer token", response.getRequestProperty("Authorization"))
        assertEquals("alex", response.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertTrue(response.closed && !response.instanceFollowRedirects)
    }

    @Test fun presentsCameraOrientationUpright() {
        val file = File.createTempFile("kinosail-photo-", ".jpg")
        try {
            file.outputStream().use {
                Bitmap.createBitmap(2, 3, Bitmap.Config.ARGB_8888)
                    .compress(Bitmap.CompressFormat.JPEG, 100, it)
            }
            ExifInterface(file.path).apply {
                setAttribute(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_ROTATE_90.toString())
                saveAttributes()
            }
            val image = ArtworkClient(server) { PhotoResponse(200, file.readBytes(), "image/jpeg") }
                .photo(item, "token", "alex")
            assertEquals(3, image.width)
            assertEquals(2, image.height)
        } finally { file.delete() }
    }

    @Test fun rejectsOtherMediaPathsAndInvalidCredentialsBeforeNetwork() {
        var opens = 0
        val client = ArtworkClient(server) { opens++; PhotoResponse(200, byteArrayOf(1)) }
        for (invalid in listOf(item.copy(stream = ""), item.copy(stream = "/media/other"),
            item.copy(stream = "https://other.example/media/photo-1"), item.copy(kind = "video"),
            item.copy(id = "../other"))) {
            assertThrows(IllegalArgumentException::class.java) { client.photo(invalid, "token", "alex") }
        }
        assertThrows(IllegalArgumentException::class.java) { client.photo(item, "bad token", "alex") }
        assertThrows(IllegalArgumentException::class.java) { client.photo(item, "token", "bad viewer") }
        assertEquals(0, opens)
    }

    @Test fun rejectsRedirectWrongTypeAndOversizedPhoto() {
        for ((index, response) in listOf(PhotoResponse(302, byteArrayOf(1)),
            PhotoResponse(200, byteArrayOf(1), "text/html"),
            PhotoResponse(200, byteArrayOf(1), length = 64L * 1024 * 1024 + 1)).withIndex()) {
            assertThrows("case $index", Exception::class.java) {
                ArtworkClient(server) { response }.photo(item, "token", "alex")
            }
            assertTrue(response.closed)
        }
    }
}

private class PhotoResponse(private val status: Int, private val body: ByteArray,
                            private val type: String = "image/png", private val length: Long = body.size.toLong()) :
    HttpURLConnection(URL("https://example.com/media/photo-1")) {
    var requestedURL: URL? = null
    var closed = false
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = type
    override fun getContentLengthLong() = length
    override fun getInputStream() = ByteArrayInputStream(body)
}
