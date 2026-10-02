package com.kinosail.player.core

import org.junit.Assert.*
import org.junit.Test

class HomeCacheScaleTest {
    @Test fun cachesTheWholeHomeWithoutStarvingListeningOnALargeMovieLibrary() {
        val items = (1..144).map { CatalogItem("item-$it", if (it > 108) "music" else "video", "Title $it", "", "", "") }
        val page = CatalogPage(items, items.size, 0, 200)
        val loaded = CatalogSnapshot.decode(CatalogSnapshot.encode(page))
        assertEquals(page, loaded)
        assertEquals(36, HomeState(recent = loaded.items, listening = true).recentShelf.size)
    }
    @Test fun rejectsOversizedAndConflictingSnapshots() {
        val item = CatalogItem("film", "video", "Film", "", "", "")
        for (page in listOf(CatalogPage(listOf(item), 0, 0, 24), CatalogPage(listOf(item), 1, 0, 0),
            CatalogPage(listOf(item), 1, 0, 201), CatalogPage(listOf(item, item), 2, 0, 24)))
            assertThrows(IllegalArgumentException::class.java) { CatalogSnapshot.encode(page) }
    }
}
