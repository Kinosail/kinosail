package com.kinosail.player.core

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyStore
import java.security.MessageDigest
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

data class SavedSession(val server: ServerAddress, val token: String, val viewer: Viewer? = null)

fun interface SessionKeyProvider { fun key(): SecretKey }

private object AndroidSessionKeyProvider : SessionKeyProvider {
    override fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey("kinosail_android_session_v1", null) as? SecretKey)?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        generator.init(KeyGenParameterSpec.Builder("kinosail_android_session_v1",
            KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
            .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
            .setKeySize(256).build())
        return generator.generateKey()
    }
}

class SessionStore(context: Context, private val keys: SessionKeyProvider = AndroidSessionKeyProvider) {
    private val preferences = context.getSharedPreferences("kinosail_session", Context.MODE_PRIVATE)

    @Synchronized
    fun save(session: SavedSession) {
        val token = ServerApi.checkedCredential(session.token, 512)
        val viewer = session.viewer?.validated()
        val plain = buildJsonObject {
            put("server", session.server.url.toString()); put("token", token)
            if (viewer != null) put("viewer", buildJsonObject {
                put("server", viewer.server); put("serverId", viewer.serverId)
                put("id", viewer.id); put("name", viewer.name)
            })
        }
            .toString().toByteArray(Charsets.UTF_8)
        require(plain.size <= 4096) { "Session is too large." }
        val encoded = encrypt(plain, AAD)
        check(preferences.edit().putString("session", encoded).commit()) { "Could not save this session." }
    }

    @Synchronized
    fun load(): SavedSession? {
        val encoded = preferences.getString("session", null) ?: return null
        val plain = decrypt(encoded, AAD, 4096)
        require(plain.size <= 4096) { "Saved session is invalid." }
        val fields = StrictJson.parse(plain.toString(Charsets.UTF_8)) as? JsonObject
        require(fields != null && fields.keys.containsAll(setOf("server", "token")) &&
            fields.keys.all { it in setOf("server", "token", "viewer") }) { "Saved session is invalid." }
        fun text(name: String): String {
            val value = fields[name] as? JsonPrimitive
            require(value != null && value.isString) { "Saved session is invalid." }
            return value.content
        }
        val viewer = fields["viewer"]?.let { raw ->
            val identity = raw as? JsonObject
            require(identity != null && identity.keys == setOf("server", "serverId", "id", "name")) {
                "Saved session is invalid."
            }
            fun value(key: String): String {
                val field = identity[key] as? JsonPrimitive
                require(field != null && field.isString) { "Saved session is invalid." }
                return field.content
            }
            Viewer(value("server"), value("serverId"), value("id"), value("name")).validated()
        }
        return SavedSession(ServerAddress(text("server")), ServerApi.checkedCredential(text("token"), 512), viewer)
    }

    @Synchronized
    fun clear() {
        check(preferences.edit().clear().commit()) { "Could not clear this session." }
    }

    @Synchronized
    fun saveCatalog(server: ServerAddress, viewer: Viewer, name: String, page: CatalogPage) {
        val entry = catalogKey(server, viewer, name)
        val encoded = encrypt(CatalogSnapshot.encode(page), entry.toByteArray(Charsets.UTF_8))
        check(preferences.edit().putString(entry, encoded).commit()) { "Could not save the library." }
    }

    @Synchronized
    fun loadCatalog(server: ServerAddress, viewer: Viewer, name: String): CatalogPage? {
        val entry = catalogKey(server, viewer, name)
        val encoded = preferences.getString(entry, null) ?: return null
        return try { CatalogSnapshot.decode(decrypt(encoded, entry.toByteArray(Charsets.UTF_8), 512 * 1024)) }
        catch (_: Exception) {
            preferences.edit().remove(entry).apply()
            null
        }
    }

    private fun catalogKey(server: ServerAddress, viewer: Viewer, name: String): String {
        viewer.validated()
        require(name in setOf("home-history", "home-recent") ||
            name.startsWith("library-") && LIBRARY_VIEWS.any { "library-${it.first}" == name }) {
            "Invalid library cache key."
        }
        val scope = "${server.url}\n${viewer.serverId}\n${viewer.id}"
        val digest = MessageDigest.getInstance("SHA-256").digest(scope.toByteArray(Charsets.UTF_8))
            .joinToString("") { "%02x".format(it) }
        return "catalog-$digest-$name"
    }

    private fun encrypt(plain: ByteArray, aad: ByteArray): String {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, keys.key())
        cipher.updateAAD(aad)
        return Base64.encodeToString(cipher.iv + cipher.doFinal(plain), Base64.NO_WRAP)
    }

    private fun decrypt(encoded: String, aad: ByteArray, maximum: Int): ByteArray {
        require(encoded.length <= (maximum + 28) * 4 / 3 + 8) { "Saved content is invalid." }
        val bytes = Base64.decode(encoded, Base64.NO_WRAP)
        require(bytes.size in 29..(maximum + 28)) { "Saved content is invalid." }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, keys.key(), GCMParameterSpec(128, bytes.copyOfRange(0, 12)))
        cipher.updateAAD(aad)
        return cipher.doFinal(bytes.copyOfRange(12, bytes.size))
    }

    companion object {
        private val AAD = "kinosail-session-v1".toByteArray(Charsets.UTF_8)
    }
}
