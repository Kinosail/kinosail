package com.kinosail.player.core

import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonArray
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.intOrNull
import kotlinx.serialization.json.put

internal object CatalogSnapshot {
    fun encode(page: CatalogPage): ByteArray {
        require(page.offset == 0 && page.limit == CatalogApi.PAGE_SIZE &&
            page.total in page.items.size..10_000_000 && page.items.size <= CatalogApi.PAGE_SIZE &&
            page.items.map(CatalogItem::id).toSet().size == page.items.size) { "Invalid catalog snapshot." }
        val raw = buildJsonObject {
            put("version", 1); put("total", page.total); put("offset", page.offset); put("limit", page.limit)
            put("items", buildJsonArray { page.items.forEach { item -> add(buildJsonObject {
                put("id", item.id); put("kind", item.kind); put("title", item.title)
                put("year", item.year); put("plot", item.plot); put("artwork", item.artwork)
                put("progress", item.progress.json()); put("showId", item.showId)
                put("season", item.season); put("episode", item.episode); put("stream", item.stream)
                put("artist", item.artist); put("album", item.album)
            }) } })
        }.toString().toByteArray(Charsets.UTF_8)
        require(raw.size <= 512 * 1024 && decode(raw) == page) { "Invalid catalog snapshot." }
        return raw
    }

    fun decode(raw: ByteArray): CatalogPage {
        require(raw.size <= 512 * 1024) { "Invalid catalog snapshot." }
        val root = StrictJson.parse(raw.toString(Charsets.UTF_8)) as? JsonObject
        require(root != null && root.keys == setOf("version", "total", "offset", "limit", "items")) {
            "Invalid catalog snapshot."
        }
        fun number(key: String): Int {
            val value = root[key] as? JsonPrimitive
            require(value != null && !value.isString) { "Invalid catalog snapshot." }
            return value.intOrNull ?: throw IllegalArgumentException("Invalid catalog snapshot.")
        }
        require(number("version") == 1 && number("offset") == 0 && number("limit") == CatalogApi.PAGE_SIZE) {
            "Invalid catalog snapshot."
        }
        val total = number("total")
        val items = root["items"] as? JsonArray
        require(items != null && items.size <= CatalogApi.PAGE_SIZE && total in items.size..10_000_000) {
            "Invalid catalog snapshot."
        }
        val parsed = items.map(CatalogApi::parseItem)
        require(parsed.map(CatalogItem::id).toSet().size == parsed.size) { "Invalid catalog snapshot." }
        return CatalogPage(parsed, total, 0, CatalogApi.PAGE_SIZE)
    }
}
