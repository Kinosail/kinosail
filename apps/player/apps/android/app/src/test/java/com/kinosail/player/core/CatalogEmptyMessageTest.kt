package com.kinosail.player.core

import org.junit.Assert.assertEquals
import org.junit.Test

class CatalogEmptyMessageTest {
    @Test fun searchWithoutMatchesDoesNotCallTheLibraryEmpty() {
        assertEquals("No results. Try another search.", catalogEmptyMessage("all", true))
        assertEquals("No results. Try another search.", catalogEmptyMessage("list", true))
    }

    @Test fun genuinelyEmptyViewsKeepTheirGuidance() {
        assertEquals("Nothing in your library yet.", catalogEmptyMessage("all", false))
        assertEquals("Save a title to keep it in My List.", catalogEmptyMessage("list", false))
    }
}
