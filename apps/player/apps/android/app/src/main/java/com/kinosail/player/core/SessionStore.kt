package com.kinosail.player.core

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

data class SavedSession(val server: ServerAddress, val token: String)

class SessionStore(context: Context) {
    private val preferences = context.getSharedPreferences("kinosail_session", Context.MODE_PRIVATE)

    @Synchronized
    fun save(session: SavedSession) {
        val token = ServerApi.checkedCredential(session.token, 512)
        val plain = buildJsonObject { put("server", session.server.url.toString()); put("token", token) }
            .toString().toByteArray(Charsets.UTF_8)
        require(plain.size <= 4096) { "Session is too large." }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, key())
        cipher.updateAAD(AAD)
        val encoded = Base64.encodeToString(cipher.iv + cipher.doFinal(plain), Base64.NO_WRAP)
        check(preferences.edit().putString("session", encoded).commit()) { "Could not save this session." }
    }

    @Synchronized
    fun load(): SavedSession? {
        val encoded = preferences.getString("session", null) ?: return null
        require(encoded.length <= 8192) { "Saved session is invalid." }
        val bytes = Base64.decode(encoded, Base64.NO_WRAP)
        require(bytes.size in 29..4096) { "Saved session is invalid." }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes.copyOfRange(0, 12)))
        cipher.updateAAD(AAD)
        val plain = cipher.doFinal(bytes.copyOfRange(12, bytes.size))
        require(plain.size <= 4096) { "Saved session is invalid." }
        val fields = StrictJson.parse(plain.toString(Charsets.UTF_8)) as? JsonObject
        require(fields != null && fields.keys == setOf("server", "token")) { "Saved session is invalid." }
        fun text(name: String): String {
            val value = fields[name] as? JsonPrimitive
            require(value != null && value.isString) { "Saved session is invalid." }
            return value.content
        }
        return SavedSession(ServerAddress(text("server")), ServerApi.checkedCredential(text("token"), 512))
    }

    @Synchronized
    fun clear() {
        check(preferences.edit().remove("session").commit()) { "Could not clear this session." }
    }

    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey(KEY_ALIAS, null) as? SecretKey)?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        generator.init(KeyGenParameterSpec.Builder(KEY_ALIAS,
            KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
            .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
            .setKeySize(256).build())
        return generator.generateKey()
    }

    companion object {
        private const val KEY_ALIAS = "kinosail_android_session_v1"
        private val AAD = "kinosail-session-v1".toByteArray(Charsets.UTF_8)
    }
}
