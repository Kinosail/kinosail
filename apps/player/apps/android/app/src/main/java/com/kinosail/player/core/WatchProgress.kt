package com.kinosail.player.core

import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.longOrNull
import kotlinx.serialization.json.put

data class WatchProgress(val seconds: Double = 0.0, val watched: Boolean = false,
                         val session: String = "", val revision: Long = 0) {
    fun validated(required: Boolean = false): WatchProgress {
        require(seconds.isFinite() && seconds in 0.0..31_536_000.0 &&
            session.toByteArray().size <= 128 && session.none(Char::isISOControl) &&
            revision in 0..9_007_199_254_740_991 &&
            (!required || session.isNotEmpty() && revision > 0)) { "Invalid watch progress." }
        return this
    }

    fun json(): JsonObject = buildJsonObject {
        put("seconds", seconds); put("watched", watched); put("session", session); put("revision", revision)
    }

    companion object {
        fun parse(raw: JsonElement): WatchProgress {
            val value = raw as? JsonObject ?: throw IllegalArgumentException("Invalid watch progress.")
            require(value.keys.all { it in setOf("seconds", "watched", "session", "revision", "updated",
                "dismissed", "readerPage", "readerOffset") }) { "Invalid watch progress." }
            fun number(key: String): Double {
                val rawNumber = value[key] ?: return 0.0
                val primitive = rawNumber as? JsonPrimitive
                require(primitive != null) { "Invalid watch progress." }
                require(!primitive.isString) { "Invalid watch progress." }
                return primitive.doubleOrNull ?: throw IllegalArgumentException("Invalid watch progress.")
            }
            fun flag(key: String): Boolean {
                val rawFlag = value[key] ?: return false
                val primitive = rawFlag as? JsonPrimitive
                require(primitive != null && !primitive.isString) { "Invalid watch progress." }
                return primitive.booleanOrNull ?: throw IllegalArgumentException("Invalid watch progress.")
            }
            val session = value["session"]?.let {
                val primitive = it as? JsonPrimitive
                require(primitive != null && primitive.isString) { "Invalid watch progress." }
                primitive.content
            } ?: ""
            val revision = value["revision"]?.let {
                val primitive = it as? JsonPrimitive
                require(primitive != null && !primitive.isString) { "Invalid watch progress." }
                primitive.longOrNull ?: throw IllegalArgumentException("Invalid watch progress.")
            } ?: 0
            val page = number("readerPage")
            val offset = number("readerOffset")
            require(page % 1 == 0.0 && page in 0.0..10_000_000.0 && offset in 0.0..1.0 &&
                (page > 0 || offset == 0.0)) { "Invalid watch progress." }
            value["updated"]?.let {
                val primitive = it as? JsonPrimitive
                require(primitive != null && primitive.isString && primitive.content.length <= 40 &&
                    primitive.content.matches(Regex("\\d{4}-\\d{2}-\\d{2}T[^\\s]{1,30}"))) { "Invalid watch progress." }
            }
            flag("dismissed")
            return WatchProgress(number("seconds"), flag("watched"), session, revision).validated()
        }
    }
}
