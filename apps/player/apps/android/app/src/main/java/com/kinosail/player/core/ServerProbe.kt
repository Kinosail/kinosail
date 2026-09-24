package com.kinosail.player.core

import java.net.HttpURLConnection
import java.net.URL

class ServerProbe(private val open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection }) {
    fun check(input: String): ServerAddress {
        val address = ServerAddress(input)
        val connection = open(URL(address.url, "healthz"))
        try {
            connection.requestMethod = "GET"
            connection.instanceFollowRedirects = false
            connection.connectTimeout = 5_000
            connection.readTimeout = 5_000
            connection.setRequestProperty("Accept", "application/json")
            require(connection.responseCode == 200 &&
                connection.contentType?.substringBefore(';')?.trim()?.lowercase() == "application/json" &&
                boundedContentLength(connection, 4096)) { "This Server did not return a valid health response." }
            val bytes = connection.inputStream.use { stream ->
                val buffer = ByteArray(4097)
                var count = 0
                while (count < buffer.size) {
                    val read = stream.read(buffer, count, buffer.size - count)
                    if (read < 0) break
                    count += read
                }
                require(count <= 4096) { "This Server response is too large." }
                buffer.copyOf(count)
            }
            require(bytes.toString(Charsets.UTF_8).trim() == "{\"status\":\"ok\"}") {
                "This Server did not return a valid health response."
            }
            return address
        } finally {
            connection.disconnect()
        }
    }
}
