package com.kinosail.player.core

import android.content.Context
import java.security.MessageDigest
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.put

data class PendingProgress(val itemId: String, val progress: WatchProgress,
                           val expected: WatchProgress, val conflict: WatchProgress? = null)

class ProgressJournal(private val read: () -> String?, private val write: (String) -> Boolean) {
    @Synchronized fun pending(): List<PendingProgress> = load()

    @Synchronized fun record(itemId: String, progress: WatchProgress, expected: WatchProgress) {
        require(itemId.matches(ID)) { "Invalid progress item." }
        progress.validated(true); expected.validated()
        val entries = load().toMutableList()
        val index = entries.indexOfFirst { it.itemId == itemId }
        if (index >= 0) {
            val old = entries[index]
            require(old.conflict == null && old.progress.session == progress.session) {
                "Resolve saved progress before recording a new session."
            }
            if (old.progress.revision >= progress.revision) return
            entries[index] = old.copy(progress = progress)
        } else {
            require(entries.size < 50) { "Too many positions are waiting to sync." }
            entries.add(PendingProgress(itemId, progress, expected))
        }
        save(entries)
    }

    @Synchronized fun apply(itemId: String, sent: WatchProgress, result: ProgressResult) {
        val entries = load().toMutableList()
        val index = entries.indexOfFirst { it.itemId == itemId }
        if (index < 0) return
        val old = entries[index]
        if (result.conflict) entries[index] = old.copy(conflict = result.progress)
        else if (old.progress == sent) entries.removeAt(index)
        else entries[index] = old.copy(expected = result.progress)
        save(entries)
    }

    @Synchronized fun resolve(itemId: String, useDevice: Boolean) {
        require(itemId.matches(ID)) { "Invalid progress item." }
        val entries = load().toMutableList()
        val index = entries.indexOfFirst { it.itemId == itemId && it.conflict != null }
        require(index >= 0) { "No progress conflict to resolve." }
        if (useDevice) entries[index] = entries[index].copy(expected = requireNotNull(entries[index].conflict), conflict = null)
        else entries.removeAt(index)
        save(entries)
    }

    private fun load(): List<PendingProgress> {
        val raw = read() ?: return emptyList()
        require(raw.toByteArray().size <= 256 * 1024) { "Saved progress is invalid." }
        val root = StrictJson.parse(raw) as? JsonObject
        require(root != null && root.keys == setOf("version", "entries") &&
            (root["version"] as? JsonPrimitive)?.let { !it.isString && it.intOrNull == 1 } == true) {
            "Saved progress is invalid."
        }
        val array = root["entries"] as? JsonArray
        require(array != null && array.size <= 50) { "Saved progress is invalid." }
        val entries = array.map { rawEntry ->
            val entry = rawEntry as? JsonObject
            require(entry != null && entry.keys.containsAll(setOf("itemId", "progress", "expected")) &&
                entry.keys.all { it in setOf("itemId", "progress", "expected", "conflict") }) {
                "Saved progress is invalid."
            }
            val id = entry["itemId"] as? JsonPrimitive
            require(id != null && id.isString && id.content.matches(ID)) { "Saved progress is invalid." }
            PendingProgress(id.content, WatchProgress.parse(entry.getValue("progress")).validated(true),
                WatchProgress.parse(entry.getValue("expected")), entry["conflict"]?.let(WatchProgress::parse))
        }
        require(entries.map(PendingProgress::itemId).toSet().size == entries.size) { "Saved progress is invalid." }
        return entries
    }

    private fun save(entries: List<PendingProgress>) {
        val raw = buildJsonObject {
            put("version", 1)
            put("entries", buildJsonArray { entries.forEach { entry -> add(buildJsonObject {
                put("itemId", entry.itemId); put("progress", entry.progress.json()); put("expected", entry.expected.json())
                entry.conflict?.let { put("conflict", it.json()) }
            }) } })
        }.toString()
        require(raw.toByteArray().size <= 256 * 1024) { "Too much progress to save." }
        check(write(raw)) { "Could not save progress." }
    }

    companion object {
        private val ID = Regex("[A-Za-z0-9_-]{1,128}")

        fun forViewer(context: Context, server: ServerAddress, viewer: Viewer): ProgressJournal {
            require(viewer.id.matches(ID) && viewer.serverId.toByteArray().size in 1..256 &&
                viewer.serverId.none(Char::isISOControl)) { "Invalid Viewer identity." }
            val raw = "${server.url}\n${viewer.serverId}\n${viewer.id}"
            val scope = MessageDigest.getInstance("SHA-256").digest(raw.toByteArray(Charsets.UTF_8))
                .joinToString("") { "%02x".format(it) }
            val prefs = context.getSharedPreferences("kinosail_progress", Context.MODE_PRIVATE)
            return ProgressJournal({ prefs.getString(scope, null) }, { prefs.edit().putString(scope, it).commit() })
        }
    }
}
