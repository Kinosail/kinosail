package com.kinosail.player.core

import java.net.HttpURLConnection
import java.net.URL
import java.io.IOException
import java.nio.ByteBuffer
import java.nio.charset.CodingErrorAction
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.put

data class ConnectChallenge(val code: String, val secret: String)
data class Viewer(val server: String, val serverId: String, val id: String, val name: String)
class ServerHttpException(val status: Int) : IOException("Server request failed ($status).")

class ServerApi(
    private val server: ServerAddress,
    private val open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection },
) {
    fun start(device: String): ConnectChallenge {
        require(device.isNotBlank() && device.length <= 80 && device.none(Char::isISOControl)) { "Invalid device name." }
        val fields = request("/api/v1/quick-connect", "POST", buildJsonObject { put("device", device) }, expected = 201)
            .fields(setOf("code", "secret"), setOf("code", "secret"))
        val code = fields.text("code", 6)
        require(code.matches(Regex("[0-9]{6}"))) { INVALID_RESPONSE }
        return ConnectChallenge(code, fields.secret("secret", 128))
    }

    fun poll(secret: String): String? {
        val response = requestWithStatus("/api/v1/quick-connect/token", "POST",
            buildJsonObject { put("secret", checkedCredential(secret, 128)) }, expected = setOf(201, 202, 404))
        if (response.first == 404) throw IllegalArgumentException("That code expired or was cancelled.")
        if (response.first == 202) {
            val fields = response.second.fields(setOf("status"), setOf("status"))
            require(fields.text("status", 20) == "pending") { INVALID_RESPONSE }
            return null
        }
        val fields = response.second.fields(setOf("token", "expiresIn"), setOf("token", "expiresIn"))
        val expires = (fields["expiresIn"] as? JsonPrimitive)?.intOrNull
        require(expires != null && expires in 1..315_360_000) { INVALID_RESPONSE }
        return fields.secret("token", 512)
    }

    fun cancel(secret: String) {
        requestWithStatus("/api/v1/quick-connect/cancel", "POST",
            buildJsonObject { put("secret", checkedCredential(secret, 128)) }, expected = setOf(204))
    }

    fun viewer(token: String): Viewer {
        val fields = request("/api/v1/me", "GET", token = checkedCredential(token, 512))
            .fields(setOf("server", "serverId", "viewer", "sso", "language", "languagePreference", "languages"),
                setOf("server", "serverId", "viewer"))
        val profile = fields.getValue("viewer").fields(
            setOf("id", "name", "owner", "downloads", "transcode", "remote", "rating", "accessStart", "accessEnd", "libraries"),
            setOf("id", "name"))
        val id = profile.text("id", 128)
        require(id.matches(Regex("[A-Za-z0-9_-]{1,128}"))) { INVALID_RESPONSE }
        for (field in listOf("owner", "downloads", "transcode", "remote")) {
            profile[field]?.let { require((it as? JsonPrimitive)?.booleanOrNull != null) { INVALID_RESPONSE } }
        }
        profile["libraries"]?.let { libraries ->
            require(libraries is JsonArray && libraries.size <= 256 && libraries.all { entry ->
                entry is JsonPrimitive && entry.isString && entry.content.toByteArray().size <= 4096
            }) { INVALID_RESPONSE }
        }
        return Viewer(fields.text("server", 120), fields.text("serverId", 256), id, profile.text("name", 120))
    }

    fun signOut(token: String) {
        requestWithStatus("/api/v1/session", "DELETE", token = checkedCredential(token, 512), expected = setOf(204))
    }

    internal fun catalog(path: String, token: String, viewerId: String): kotlinx.serialization.json.JsonElement {
        require(path.startsWith("/api/v1/library?") && path.length <= 2048 &&
            viewerId.matches(Regex("[A-Za-z0-9_-]{1,128}"))) { "Invalid catalog request." }
        return requestWithStatus(path, "GET", token = checkedCredential(token, 512),
            viewerId = viewerId, expected = setOf(200), maximum = 2 * 1024 * 1024).second
    }

    internal fun playback(path: String, token: String, viewerId: String): kotlinx.serialization.json.JsonElement {
        require(path.matches(Regex("/api/v1/items/[A-Za-z0-9_-]{1,128}/playback\\?[A-Za-z0-9=,&_-]{1,512}")) &&
            viewerId.matches(Regex("[A-Za-z0-9_-]{1,128}"))) { "Invalid playback request." }
        return requestWithStatus(path, "GET", token = checkedCredential(token, 512),
            viewerId = viewerId, expected = setOf(200), maximum = 2 * 1024 * 1024).second
    }

    internal fun syncProgress(itemId: String, token: String, viewerId: String, body: JsonObject):
        Pair<Int, kotlinx.serialization.json.JsonElement> {
        require(itemId.matches(Regex("[A-Za-z0-9_-]{1,128}")) &&
            viewerId.matches(Regex("[A-Za-z0-9_-]{1,128}"))) { "Invalid progress request." }
        return requestWithStatus("/api/v1/items/$itemId/progress/sync", "PUT", body,
            token = checkedCredential(token, 512), viewerId = viewerId, expected = setOf(200, 409))
    }

    internal fun item(itemId: String, token: String, viewerId: String): kotlinx.serialization.json.JsonElement {
        require(itemId.matches(Regex("[A-Za-z0-9_-]{1,128}")) &&
            viewerId.matches(Regex("[A-Za-z0-9_-]{1,128}"))) { "Invalid item request." }
        return requestWithStatus("/api/v1/items/$itemId", "GET", token = checkedCredential(token, 512),
            viewerId = viewerId, expected = setOf(200), maximum = 2 * 1024 * 1024).second
    }

    private fun request(path: String, method: String, body: JsonObject? = null, token: String? = null,
                        expected: Int = 200): kotlinx.serialization.json.JsonElement =
        requestWithStatus(path, method, body, token, expected = setOf(expected)).second

    private fun requestWithStatus(path: String, method: String, body: JsonObject? = null, token: String? = null,
                                  viewerId: String? = null, expected: Set<Int>, maximum: Int = 65_536
    ): Pair<Int, kotlinx.serialization.json.JsonElement> {
        val bytes = body?.toString()?.toByteArray(Charsets.UTF_8)
        require(bytes == null || bytes.size <= 4096) { "Request is too large." }
        val connection = open(URL(server.url, path))
        try {
            connection.requestMethod = method
            connection.instanceFollowRedirects = false
            connection.connectTimeout = 10_000
            connection.readTimeout = 20_000
            connection.useCaches = false
            connection.setRequestProperty("Accept", "application/json")
            if (token != null) connection.setRequestProperty("Authorization", "Bearer $token")
            if (viewerId != null) connection.setRequestProperty("X-Kinosail-Viewer-Profile", viewerId)
            if (bytes != null) {
                connection.doOutput = true
                connection.setRequestProperty("Content-Type", "application/json")
                connection.outputStream.use { it.write(bytes) }
            }
            val status = connection.responseCode
            if (status !in expected) throw ServerHttpException(status)
            if (status == 204 || status == 404) return status to kotlinx.serialization.json.JsonNull
            require(connection.contentType?.substringBefore(';')?.trim()?.lowercase() == "application/json" &&
                connection.contentLengthLong <= maximum) { INVALID_RESPONSE }
            val response = (if (status >= 400) connection.errorStream
                ?: throw IOException(INVALID_RESPONSE) else connection.inputStream).use { stream ->
                val buffer = ByteArray(maximum + 1)
                var count = 0
                while (count < buffer.size) {
                    val read = stream.read(buffer, count, buffer.size - count)
                    if (read < 0) break
                    count += read
                }
                require(count <= maximum) { INVALID_RESPONSE }
                buffer.copyOf(count)
            }
            val decoded = Charsets.UTF_8.newDecoder().onMalformedInput(CodingErrorAction.REPORT)
                .decode(ByteBuffer.wrap(response)).toString()
            return status to StrictJson.parse(decoded)
        } finally {
            connection.disconnect()
        }
    }

    companion object {
        private const val INVALID_RESPONSE = "The Server returned an invalid response."

        fun checkedCredential(value: String, max: Int): String {
            require(value.isNotEmpty() && value.length <= max && value.all { it.code in 33..126 }) {
                "Invalid credential."
            }
            return value
        }

        private fun kotlinx.serialization.json.JsonElement.fields(allowed: Set<String>, required: Set<String>): JsonObject {
            val fields = this as? JsonObject
            require(fields != null && fields.keys.all(allowed::contains) && fields.keys.containsAll(required)) { INVALID_RESPONSE }
            return fields
        }

        private fun JsonObject.text(key: String, max: Int): String {
            val value = this[key] as? JsonPrimitive
            require(value != null && value.isString && value.content.isNotEmpty() &&
                value.content.toByteArray(Charsets.UTF_8).size <= max && value.content.none(Char::isISOControl)) {
                INVALID_RESPONSE
            }
            return value.content
        }

        private fun JsonObject.secret(key: String, max: Int): String = checkedCredential(text(key, max), max)
    }
}
