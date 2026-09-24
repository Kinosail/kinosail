package com.kinosail.player.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class ProgressJournalTest {
    private val expected = WatchProgress(8.0, false, "old", 2)
    private val first = WatchProgress(20.0, false, "new", 1)

    @Test fun savesBeforeSyncAndKeepsNewerPositions() {
        var disk: String? = null
        val journal = ProgressJournal({ disk }, { disk = it; true })
        journal.record("film-1", first, expected)
        assertTrue(disk!!.contains("film-1"))
        val reloaded = ProgressJournal({ disk }, { disk = it; true })
        assertEquals(first, reloaded.pending().single().progress)
        reloaded.record("film-1", first.copy(seconds = 30.0, revision = 2), expected)
        reloaded.apply("film-1", first, ProgressResult(first, false))
        assertEquals(30.0, reloaded.pending().single().progress.seconds, 0.0)
        assertEquals(first, reloaded.pending().single().expected)
        reloaded.apply("film-1", first.copy(seconds = 30.0, revision = 2),
            ProgressResult(first.copy(seconds = 30.0, revision = 2), false))
        assertTrue(reloaded.pending().isEmpty())
    }

    @Test fun retainsConflictsUntilExplicitResolution() {
        var disk: String? = null
        val journal = ProgressJournal({ disk }, { disk = it; true })
        journal.record("film-1", first, expected)
        val remote = WatchProgress(55.0, true, "other", 3)
        journal.apply("film-1", first, ProgressResult(remote, true))
        assertEquals(remote, journal.pending().single().conflict)
        assertThrows(IllegalArgumentException::class.java) {
            journal.record("film-1", first.copy(revision = 2), expected)
        }
        journal.resolve("film-1", true)
        assertEquals(remote, journal.pending().single().expected)
        journal.apply("film-1", first, ProgressResult(first, false))
        assertTrue(journal.pending().isEmpty())
        journal.record("film-1", first, expected)
        journal.apply("film-1", first, ProgressResult(remote, true))
        journal.resolve("film-1", false)
        assertTrue(journal.pending().isEmpty())
    }

    @Test fun rejectsMalformedOversizedAndDuplicateStoredEntriesWithoutWrites() {
        var disk: String? = null
        var writes = 0
        val journal = ProgressJournal({ disk }, { disk = it; writes++; true })
        val bad = listOf("{}", "x".repeat(256 * 1024 + 1),
            """{"version":2,"entries":[]}""",
            """{"version":1,"entries":[{"itemId":"../x","progress":{},"expected":{}}]}""",
            """{"version":1,"entries":[{"itemId":"film-1","progress":{"seconds":-1,"session":"s","revision":1},"expected":{}}]}""",
            """{"version":1,"entries":[{"itemId":"film-1","progress":{"session":"s","revision":1},"expected":{}},{"itemId":"film-1","progress":{"session":"s","revision":1},"expected":{}}]}""")
        bad.forEach {
            disk = it
            assertThrows(it.take(40), Exception::class.java) { journal.record("film-2", first, expected) }
        }
        assertEquals(0, writes)
        disk = null
        assertThrows(IllegalArgumentException::class.java) { journal.record("bad/id", first, expected) }
        assertThrows(IllegalArgumentException::class.java) { journal.record("film-1", first.copy(seconds = -1.0), expected) }
        assertEquals(0, writes)
    }

    @Test fun storageFailureDoesNotPretendToRecord() {
        val journal = ProgressJournal({ null }, { false })
        assertThrows(IllegalStateException::class.java) { journal.record("film-1", first, expected) }
        assertTrue(journal.pending().isEmpty())
    }
}
