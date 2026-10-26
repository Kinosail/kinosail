package com.kinosail.player.core

import android.graphics.Bitmap
import android.util.LruCache
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

internal class CatalogImages {
    private val cache = object : LruCache<String, Bitmap>(maxOf(24 * 1024 * 1024, (Runtime.getRuntime().maxMemory() / 6).toInt())) {
        override fun sizeOf(key: String, value: Bitmap): Int = value.byteCount
    }

    fun clear() = cache.evictAll()

    suspend fun artwork(saved: SavedSession, viewer: String, path: String, dimension: Int, current: () -> Boolean): Bitmap? {
        if (path.isEmpty()) return null
        val key = "$viewer:$dimension:$path"
        cache.get(key)?.let { return it }
        return try {
            val image = withContext(Dispatchers.IO) { ArtworkClient(saved.server).bitmap(path, saved.token, viewer, dimension) }
            if (!current()) null else image.also { cache.put(key, it) }
        } catch (_: Exception) { null }
    }

    suspend fun photo(saved: SavedSession, viewer: String, item: CatalogItem, current: () -> Boolean): Bitmap? = try {
        withContext(Dispatchers.IO) { ArtworkClient(saved.server).photo(item, saved.token, viewer) }.takeIf { current() }
    } catch (_: Exception) { null }
}
