package com.kinosail.player.core

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class PlaybackPreferencesApiTest {
    private val server = ServerAddress("https://example.com")
    private val preferences = PlaybackPreferences("English", "", 1.0, "auto", "off", false, true, 1.0)
    private fun response(value: PlaybackPreferences = preferences, overridden: Boolean = false) =
        """{"playback":${value.json()},"overridden":$overridden}"""

    @Test fun loadsViewerScopedPreferencesAndPreservesEverySettingWhenSavingSpeed() {
        val loaded = PreferenceResponse(200, response())
        val api = PlaybackPreferencesApi(server) { url -> loaded.also { it.requestedURL = url } }
        assertEquals(preferences, api.load("film-1", "token", "alex"))
        assertEquals("GET", loaded.requestMethod)
        assertEquals("/api/v1/items/film-1/playback-preferences", loaded.requestedURL.path)
        assertEquals("Bearer token", loaded.getRequestProperty("Authorization"))
        assertEquals("alex", loaded.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertTrue(loaded.closed)
        assertFalse(loaded.instanceFollowRedirects)

        val changed = preferences.copy(rate = 1.25)
        val saved = PreferenceResponse(200, response(changed, overridden = true))
        assertEquals(changed, PlaybackPreferencesApi(server) { saved }.save("film-1", "token", "alex", changed))
        assertEquals("PUT", saved.requestMethod)
        assertEquals(changed.json().toString(), saved.sent.toString(Charsets.UTF_8.name()))
        assertEquals("alex", saved.getRequestProperty("X-Kinosail-Viewer-Profile"))
        assertTrue(saved.closed)
    }

    @Test fun rejectsInvalidInputsBeforeOpeningConnection() {
        var opens = 0
        val api = PlaybackPreferencesApi(server) { opens++; PreferenceResponse(200, response()) }
        assertThrows(IllegalArgumentException::class.java) { api.load("../film", "token", "alex") }
        assertThrows(IllegalArgumentException::class.java) { api.load("film-1", "bad token", "alex") }
        assertThrows(IllegalArgumentException::class.java) { api.load("film-1", "token", "bad viewer") }
        listOf(0.0, 0.49, 3.01, Double.NaN, Double.POSITIVE_INFINITY).forEach { rate ->
            assertThrows(IllegalArgumentException::class.java) { preferences.copy(rate = rate) }
        }
        assertThrows(IllegalArgumentException::class.java) { preferences.copy(audioTrack = "x".repeat(257)) }
        assertThrows(IllegalArgumentException::class.java) { preferences.copy(audioLanguage = "invalid language") }
        assertThrows(IllegalArgumentException::class.java) { preferences.copy(volumeBoost = 2.1) }
        assertEquals(0, opens)
    }

    @Test fun rejectsMalformedResponsesAndConflictingSavedValues() {
        val valid = response()
        listOf("{}", valid.replace("\"rate\":1.0", "\"rate\":\"1\""),
            valid.replace("\"overridden\":false", "\"overridden\":\"false\""),
            valid.replace("\"audioTrack\":\"English\"", "\"audioTrack\":\"${"x".repeat(257)}\""),
            valid.replace("\"volumeBoost\":1.0", "\"volumeBoost\":3.0"),
            valid.replace("\"subtitleLanguage\":\"off\"", "\"subtitleLanguage\":\"bad language\""),
            valid.replace("\"nightMode\":false", "\"nightMode\":0"),
            valid.replace("\"dialogueBoost\":true", "\"dialogueBoost\":null"),
            valid.replace("\"playback\":", "\"extra\":true,\"playback\":"),
            valid.replace("\"rate\":1.0", "\"rate\":1.0,\"rate\":2.0"))
            .forEach { body -> assertThrows(body, Exception::class.java) {
                PlaybackPreferencesApi(server) { PreferenceResponse(200, body) }.load("film-1", "token", "alex")
            } }
        val changed = preferences.copy(rate = 1.25)
        listOf(response(), response(changed, overridden = false)).forEach { body ->
            assertThrows(IllegalArgumentException::class.java) {
                PlaybackPreferencesApi(server) { PreferenceResponse(200, body) }
                    .save("film-1", "token", "alex", changed)
            }
        }
    }

    @Test fun rejectsRedirectAndWrongContentType() {
        listOf(PreferenceResponse(302, response()), PreferenceResponse(200, response(), "text/html"))
            .forEach { connection ->
                assertThrows(Exception::class.java) {
                    PlaybackPreferencesApi(server) { connection }.load("film-1", "token", "alex")
                }
                assertTrue(connection.closed)
            }
    }
}

private class PreferenceResponse(private val status: Int, private val body: String,
                                 private val type: String = "application/json") :
    HttpURLConnection(URL("https://example.com/api/v1/items/film-1/playback-preferences")) {
    lateinit var requestedURL: URL
    var closed = false
    val sent = ByteArrayOutputStream()
    override fun connect() = Unit
    override fun disconnect() { closed = true }
    override fun usingProxy() = false
    override fun getResponseCode() = status
    override fun getContentType() = type
    override fun getContentLengthLong() = body.toByteArray().size.toLong()
    override fun getInputStream() = ByteArrayInputStream(body.toByteArray())
    override fun getOutputStream() = sent
}
