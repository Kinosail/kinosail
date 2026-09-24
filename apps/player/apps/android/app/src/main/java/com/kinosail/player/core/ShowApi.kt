package com.kinosail.player.core

import java.net.HttpURLConnection
import java.net.URL
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

data class ShowDetail(val id: String, val title: String, val year: String, val plot: String,
                      val episodes: List<CatalogItem>) {
    val next: CatalogItem? get() = episodes.firstOrNull { !it.progress.watched } ?: episodes.firstOrNull()
    val seasons: List<Int> get() = episodes.map(CatalogItem::season).distinct().sorted()
}

class ShowApi(server: ServerAddress,
              open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection }) {
    private val api = ServerApi(server, open)

    fun detail(showId: String, token: String, viewerId: String): ShowDetail {
        val root = api.show(showId, token, viewerId) as? JsonObject
        require(root != null && root.keys.all { it in setOf("id", "title", "backdrop", "play", "episodes",
            "cast", "year", "plot", "genres", "studio") } && root.keys.containsAll(setOf("id", "title", "episodes"))) {
            "Invalid show response."
        }
        fun text(key: String, max: Int, required: Boolean = false): String {
            val raw = root[key] ?: return if (required) throw IllegalArgumentException("Invalid show response.") else ""
            val primitive = raw as? JsonPrimitive
            require(primitive != null && primitive.isString && (!required || primitive.content.isNotEmpty()) &&
                primitive.content.toByteArray().size <= max && primitive.content.none(Char::isISOControl)) {
                "Invalid show response."
            }
            return primitive.content
        }
        require(text("id", 16) == showId) { "Invalid show response." }
        val rawEpisodes = root["episodes"] as? JsonArray
        require(rawEpisodes != null && rawEpisodes.size <= 10_000) { "Invalid show response." }
        val episodes = rawEpisodes.map(CatalogApi::parseItem)
        require(episodes.all { it.kind == "video" && it.showId == showId } &&
            episodes.map(CatalogItem::id).toSet().size == episodes.size) { "Invalid show response." }
        return ShowDetail(showId, text("title", 512, true), text("year", 16), text("plot", 10_000),
            episodes.sortedWith(compareBy(CatalogItem::season, CatalogItem::episode, CatalogItem::title)))
    }
}
