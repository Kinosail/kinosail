package com.kinosail.player.core

import kotlinx.serialization.json.Json
import org.junit.Assert.*
import org.junit.Test

class NativeMetadataTest {
    private fun parse(extra: String) = CatalogApi.parseItem(Json.parseToJsonElement(
        """{"id":"film","kind":"video","title":"Film",$extra}"""))

    @Test fun preservesHeroMetadataAndDismissalAcrossCachedLaunches() {
        val item = parse(""""backdrop":"/backdrop/film","rating":"PG-13","genres":"Drama · Mystery","progress":{"seconds":42,"dismissed":true}""")
        assertEquals("/backdrop/film", item.backdrop)
        assertEquals("PG-13", item.rating)
        assertEquals("Drama · Mystery", item.genres)
        assertTrue(item.progress.dismissed)
        val page = CatalogPage(listOf(item), 1, 0, 24)
        assertEquals(page, CatalogSnapshot.decode(CatalogSnapshot.encode(page)))
    }

    @Test fun rejectsRemoteOriginsTraversalQueriesOversizedAndMalformedMetadata() {
        listOf("https://example.com/backdrop/film", "/backdrop/../film", "/backdrop/film?x=1", "/art/film").forEach {
            assertThrows(IllegalArgumentException::class.java) { parse(""""backdrop":"$it"""") }
        }
        listOf("\"rating\":123", "\"rating\":\"" + "x".repeat(129) + "\"",
            "\"genres\":[]", "\"genres\":\"" + "x".repeat(2049) + "\"", "\"backdrop\":null").forEach {
            assertThrows(IllegalArgumentException::class.java) { parse(it) }
        }
    }
}
