package com.kinosail.player.core

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.*
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class) @Config(sdk = [35])
class PersonalTabsTest {
    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val viewer = Viewer("Home", "server-1", "alex", "Alex")

    @Test fun persistsOneToFourUniqueTabsPerServerAndViewer() {
        val store = PersonalTabs(context)
        store.save(viewer, listOf("listen", "list"))
        assertEquals(listOf("listen", "list"), PersonalTabs(context).load(viewer))
        assertEquals(PersonalTabs.defaults, store.load(viewer.copy(id = "sam")))
        assertEquals(PersonalTabs.defaults, store.load(viewer.copy(serverId = "server-2")))
    }

    @Test fun rejectsMissingUnknownDuplicateOversizedAndMalformedTabsWithoutChangingSavedChoices() {
        val store = PersonalTabs(context)
        store.save(viewer, listOf("home"))
        listOf(emptyList(), listOf("more"), listOf("home", "home"), listOf("HOME"),
            listOf("home\n"), listOf("x".repeat(5000)), PersonalTabs.defaults + "music").forEach { value ->
            assertThrows(IllegalArgumentException::class.java) { store.save(viewer, value) }
            assertEquals(listOf("home"), store.load(viewer))
        }
        assertThrows(IllegalArgumentException::class.java) { store.save(viewer.copy(id = ""), listOf("home")) }
        assertEquals(listOf("home"), store.load(viewer))
    }
}
