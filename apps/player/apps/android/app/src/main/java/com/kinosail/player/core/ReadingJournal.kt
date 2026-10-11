package com.kinosail.player.core

import android.content.Context
import kotlinx.serialization.json.*
import java.security.MessageDigest

internal data class PendingReading(val position: ReaderPosition, val baseline: ReaderPosition)

/** Positions only; books, credentials and addresses are never stored here. */
internal class ReadingJournal(context: Context, saved: SavedSession, viewer: Viewer, itemId: String) {
    private val preferences = context.getSharedPreferences("kinosail_reading_pending", Context.MODE_PRIVATE)
    private val key = MessageDigest.getInstance("SHA-256")
        .digest("${saved.server.url}\n${viewer.serverId}\n${viewer.id}\n$itemId".toByteArray())
        .joinToString("") { "%02x".format(it) }

    fun load(total: Int, includeAcknowledged: Boolean = false): PendingReading? = synchronized(lock) {
        val raw = preferences.getString(key, null) ?: return null
        require(raw.length <= 512)
        val fields = StrictJson.parse(raw) as? JsonObject
        require(fields != null && fields.keys == setOf("position", "baseline"))
        fun position(key: String): ReaderPosition {
            val value = fields[key] as? JsonObject
            require(value != null && value.keys == setOf("page", "total", "offset"))
            val page = (value["page"] as? JsonPrimitive)?.takeUnless { it.isString }?.intOrNull
            val count = (value["total"] as? JsonPrimitive)?.takeUnless { it.isString }?.intOrNull
            val offset = (value["offset"] as? JsonPrimitive)?.takeUnless { it.isString }?.doubleOrNull
            require(count != null && count in 1..10_000 && page != null && page in 1..count && offset != null && offset.isFinite() && offset in 0.0..1.0)
            return ReaderPosition(page, count, offset)
        }
        val pending = PendingReading(position("position"), position("baseline"))
        if (pending.position.total != total || pending.baseline.total != total) {
            android.util.Log.i("KinosailReader", "Book changed operation=reader-progress kind=chapter_count outcome=server_position")
            clear()
            return null
        }
        return pending.takeIf { includeAcknowledged || it.position != it.baseline }
    }
    fun stage(position: ReaderPosition, baseline: ReaderPosition) = synchronized(lock) {
        store(PendingReading(position, load(position.total, true)?.baseline ?: baseline))
    }
    fun store(pending: PendingReading) = synchronized(lock) {
        fun json(position: ReaderPosition) = buildJsonObject {
            put("page", position.page); put("total", position.total); put("offset", position.offset)
        }
        check(preferences.edit().putString(key, buildJsonObject {
            put("position", json(pending.position)); put("baseline", json(pending.baseline))
        }.toString()).commit()) { "Could not save reading position." }
    }
    fun acknowledge(snapshot: PendingReading) = synchronized(lock) {
        val latest = load(snapshot.position.total, includeAcknowledged = true) ?: return
        if (latest == snapshot) store(PendingReading(snapshot.position, snapshot.position))
        else if (latest.baseline == snapshot.baseline) store(latest.copy(baseline = snapshot.position))
    }
    fun clear() = synchronized(lock) { check(preferences.edit().remove(key).commit()) { "Could not save reading position." } }
    companion object { private val lock = Any() }
}
