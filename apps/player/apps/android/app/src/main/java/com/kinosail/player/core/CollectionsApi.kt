package com.kinosail.player.core

import android.util.Log
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

internal class CollectionsApi(server: ServerAddress,
    open: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection }) {
    private val api = ServerApi(server, open)

    fun names(token: String, viewerId: String): List<String> = validate {
        val fields = api.collections(null, token, viewerId) as? JsonObject
        require(fields != null && fields.keys.all { it in setOf("collections", "summaries") })
        val values = fields["collections"] as? JsonArray
        require(values != null && values.size <= 10_000)
        val names = values.map { raw ->
            val name = raw as? JsonPrimitive
            require(name != null && name.isString)
            checkedName(name.content)
        }
        require(names.distinct().size == names.size)
        names
    }

    fun items(name: String, token: String, viewerId: String): List<CatalogItem> = validate {
        val fields = api.collections(checkedName(name), token, viewerId) as? JsonObject
        require(fields != null && fields.keys == setOf("name", "items"))
        require((fields["name"] as? JsonPrimitive)?.let { it.isString && it.content == name } == true)
        val values = fields["items"] as? JsonArray
        require(values != null && values.size <= 10_000)
        val items = values.map(CatalogApi::parseItem)
        require(items.map(CatalogItem::id).distinct().size == items.size)
        items
    }

    private fun <T> validate(action: () -> T): T = try { action() }
    catch (failure: IllegalArgumentException) {
            Log.w("KinosailNetwork", "Collection rejected operation=collections kind=invalid_response outcome=discarded")
        throw failure
    }

    companion object {
        fun checkedName(name: String): String {
            require(name.isNotBlank() && name == name.trim() && name.toByteArray(Charsets.UTF_8).size <= 64 &&
                name.none(Char::isISOControl) && '\\' !in name &&
                name.split('/').none { it == "." || it == ".." }) { "Invalid collection name." }
            return name
        }
        fun segment(name: String): String = URLEncoder.encode(checkedName(name), "UTF-8")
            .replace("+", "%20").replace("*", "%2A").replace("%7E", "~")
    }
}
