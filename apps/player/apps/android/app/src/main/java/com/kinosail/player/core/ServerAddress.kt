package com.kinosail.player.core

import java.net.Inet6Address
import java.net.InetAddress
import java.net.URI
import java.net.URL
import java.util.Locale

class ServerAddress(input: String) {
    val url: URL

    init {
        val raw = input.trim()
        require(raw.isNotEmpty() && raw.toByteArray(Charsets.UTF_8).size <= 2048 &&
            raw.none { it.isISOControl() || it == '\\' }) { INVALID_ADDRESS }
        val uri = try { URI(raw) } catch (_: Exception) { throw IllegalArgumentException(INVALID_ADDRESS) }
        val scheme = uri.scheme?.lowercase(Locale.ROOT)
        val host = uri.host?.lowercase(Locale.ROOT)
        require((scheme == "https" || scheme == "http") && host != null &&
            uri.userInfo == null && uri.rawQuery == null && uri.rawFragment == null &&
            (uri.rawPath.isNullOrEmpty() || uri.rawPath == "/") &&
            (uri.port == -1 || uri.port in 1..65535) &&
            (scheme != "http" || isLocalHost(host))) { INVALID_ADDRESS }
        val port = if (uri.port == (if (scheme == "https") 443 else 80)) -1 else uri.port
        url = URI(scheme, null, host, port, "/", null, null).toURL()
    }

    companion object {
        const val INVALID_ADDRESS = "Enter a valid Server URL. HTTP requires a local IP address or localhost."

        private fun isLocalHost(host: String): Boolean {
            if (host == "localhost") return true
            val octets = host.split('.')
            if (octets.size == 4 && octets.all { it.isNotEmpty() && it.length <= 3 &&
                    (it == "0" || it[0] != '0') && it.all(Char::isDigit) && it.toInt() <= 255 }) {
                val first = octets[0].toInt()
                val second = octets[1].toInt()
                return first == 10 || first == 127 || first == 192 && second == 168 ||
                    first == 172 && second in 16..31 || first == 169 && second == 254
            }
            val literal = host.removePrefix("[").removeSuffix("]")
            if (':' !in literal || !literal.all { it in "0123456789abcdef:." }) return false
            val address = try { InetAddress.getByName(literal) } catch (_: Exception) { return false }
            if (address !is Inet6Address) return false
            val first = address.address[0].toInt() and 0xff
            return address.isLoopbackAddress || address.isLinkLocalAddress || first and 0xfe == 0xfc
        }
    }
}
