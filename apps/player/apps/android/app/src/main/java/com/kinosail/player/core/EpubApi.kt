package com.kinosail.player.core

import android.util.Log
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.intOrNull

internal data class EpubChapter(val title: String, val path: String)
internal data class EpubBook(val id: String, val title: String, val chapters: List<EpubChapter>)

internal fun ReaderApi.epub(itemId: String, saved: SavedSession, viewer: Viewer): EpubBook {
    require(saved.viewer?.let { it.serverId == viewer.serverId && it.id == viewer.id } == true)
    val raw = ServerApi(saved.server).reader(itemId, saved.token, viewer.id)
    try {
        val fields = raw as? JsonObject
        require(fields != null && fields.keys == setOf("id", "title", "type", "pages"))
        fun JsonObject.text(key: String, max: Int, empty: Boolean = false): String {
            val value = this[key] as? JsonPrimitive
            require(value != null && value.isString && (empty || value.content.isNotEmpty()) &&
                value.content.toByteArray(Charsets.UTF_8).size <= max && value.content.none(Char::isISOControl))
            return value.content
        }
        require(fields.text("id", 128) == itemId && fields.text("type", 16) == "epub")
        val pages = fields["pages"] as? JsonArray
        require(pages != null && pages.size in 1..10_000)
        val chapters = pages.mapIndexed { index, rawPage ->
            val page = rawPage as? JsonObject
            require(page != null && page.keys == setOf("number", "title", "url"))
            require((page["number"] as? JsonPrimitive)?.let { !it.isString && it.intOrNull == index + 1 } == true)
            val path = page.text("url", 2048)
            require(ReaderApi.validComicPath(path, itemId))
            EpubChapter(page.text("title", 512, empty = true), path)
        }
        require(chapters.map(EpubChapter::path).distinct().size == chapters.size)
        return EpubBook(itemId, fields.text("title", 512), chapters)
    } catch (failure: IllegalArgumentException) {
        Log.w("KinosailReader", "Book rejected operation=reader-manifest kind=invalid_response outcome=discarded")
        throw failure
    }
}
