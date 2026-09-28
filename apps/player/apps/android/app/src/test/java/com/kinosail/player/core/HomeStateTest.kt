package com.kinosail.player.core

import org.junit.Assert.assertEquals
import org.junit.Test

class HomeStateTest {
    private fun item(id: String, kind: String = "video") =
        CatalogItem(id, kind, id, "", "", "")

    @Test fun featuredTitleDoesNotRepeatInNearbyShelves() {
        val featured = item("featured")
        val next = item("next")
        val recent = item("recent")
        val state = HomeState(
            continueWatching = listOf(featured, next),
            recent = listOf(recent, featured),
        )

        assertEquals(featured, state.featured)
        assertEquals(listOf(next), state.watchShelf)
        assertEquals(listOf(recent), state.recentShelf)
    }

    @Test fun recentFeatureLeavesNoDuplicateOrEmptyShelf() {
        val featured = item("featured")
        val state = HomeState(recent = listOf(featured))

        assertEquals(featured, state.featured)
        assertEquals(emptyList<CatalogItem>(), state.watchShelf)
        assertEquals(emptyList<CatalogItem>(), state.recentShelf)
    }

    @Test fun nonPlayableItemsRemainAvailableInTheirShelves() {
        val photo = item("photo", "photo")
        val movie = item("movie")
        val state = HomeState(continueWatching = listOf(photo, movie), recent = listOf(photo))

        assertEquals(movie, state.featured)
        assertEquals(listOf(photo), state.watchShelf)
        assertEquals(listOf(photo), state.recentShelf)
    }
}
