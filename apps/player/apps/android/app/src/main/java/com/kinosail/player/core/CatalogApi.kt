package com.kinosail.player.core

import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.intOrNull

data class CatalogItem(
    val id: String,
    val kind: String,
    val title: String,
    val year: String,
    val plot: String,
    val artwork: String,
)

data class CatalogPage(val items: List<CatalogItem>, val total: Int, val offset: Int, val limit: Int)

class CatalogApi(
    server: ServerAddress,
    private val open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection },
) {
    private val api = ServerApi(server, open)

    fun list(token: String, viewerId: String, rawQuery: String = "", offset: Int = 0): CatalogPage {
        val query = rawQuery.trim()
        require(query.toByteArray(Charsets.UTF_8).size <= 512 && query.none(Char::isISOControl) &&
            offset in 0..1_000_000) { "Invalid library request." }
        val encoded = URLEncoder.encode(query, Charsets.UTF_8.name())
        val result = api.catalog("/api/v1/library?q=$encoded&view=all&sort=title&offset=$offset&limit=$PAGE_SIZE",
            token, viewerId).fields(PAGE_KEYS, setOf("items", "total", "offset", "limit"))
        val total = result.number("total", 0..10_000_000)
        val returnedOffset = result.number("offset", 0..1_000_000)
        val limit = result.number("limit", 1..200)
        require(returnedOffset == offset && limit == PAGE_SIZE) { INVALID_RESPONSE }
        result["view"]?.let { require(result.text("view", 32) == "all") { INVALID_RESPONSE } }
        result["sort"]?.let { require(result.text("sort", 32) == "title") { INVALID_RESPONSE } }
        result["query"]?.let { require(result.text("query", 512, empty = true) == query) { INVALID_RESPONSE } }
        val rawItems = result["items"] as? JsonArray
        require(rawItems != null && rawItems.size <= PAGE_SIZE && rawItems.size <= maxOf(0, total - offset) &&
            (offset >= total || rawItems.isNotEmpty())) {
            INVALID_RESPONSE
        }
        val items = rawItems.map { raw ->
            val item = raw.fields(ITEM_KEYS, setOf("id", "kind", "title"))
            val id = item.text("id", 128)
            require(id.matches(ID)) { INVALID_RESPONSE }
            val kind = item.text("kind", 32).let { if (it == "audio") "music" else it }
            require(kind in KINDS) { INVALID_RESPONSE }
            val artwork = item.text("artwork", 16_384, empty = true)
            require(artwork.isEmpty() || artwork.matches(ARTWORK)) { INVALID_RESPONSE }
            CatalogItem(id, kind, item.text("title", 512), item.text("year", 16, empty = true),
                item.text("plot", 10_000, empty = true).replace("\u200B", ""), artwork)
        }
        require(items.map(CatalogItem::id).toSet().size == items.size) { INVALID_RESPONSE }
        return CatalogPage(items, total, returnedOffset, limit)
    }

    companion object {
        const val PAGE_SIZE = 24
        private const val INVALID_RESPONSE = "The Server returned an invalid library page."
        private val ID = Regex("[A-Za-z0-9_-]{1,128}")
        internal val ARTWORK = Regex("/art/[A-Za-z0-9_-]{1,128}(\\?variant=episode)?")
        private val KINDS = setOf("video", "show", "music", "audiobook", "book", "photo")
        private val PAGE_KEYS = setOf("items", "view", "sort", "query", "letter", "total", "offset", "limit", "letters")
        private val ITEM_KEYS = setOf("id", "kind", "title", "sortTitle", "year", "plot", "rating", "tagline",
            "genres", "director", "studio", "artist", "album", "track", "show", "showId", "size", "season",
            "episode", "stream", "download", "subtitles", "added", "cast", "artwork", "backdrop", "container", "progress")

        private fun JsonElement.fields(allowed: Set<String>, required: Set<String>): JsonObject {
            val fields = this as? JsonObject
            require(fields != null && fields.keys.all(allowed::contains) && fields.keys.containsAll(required)) {
                INVALID_RESPONSE
            }
            return fields
        }

        private fun JsonObject.number(key: String, range: IntRange): Int {
            val value = (this[key] as? JsonPrimitive)?.intOrNull
            require(value != null && value in range) { INVALID_RESPONSE }
            return value
        }

        private fun JsonObject.text(key: String, maximum: Int, empty: Boolean = false): String {
            val value = this[key] ?: return if (empty) "" else throw IllegalArgumentException(INVALID_RESPONSE)
            val primitive = value as? JsonPrimitive
            require(primitive != null && primitive.isString && (empty || primitive.content.isNotEmpty()) &&
                primitive.content.toByteArray(Charsets.UTF_8).size <= maximum &&
                primitive.content.none(Char::isISOControl)) { INVALID_RESPONSE }
            return primitive.content
        }
    }
}
