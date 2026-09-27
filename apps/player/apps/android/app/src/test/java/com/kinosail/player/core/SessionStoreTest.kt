package com.kinosail.player.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config
import javax.crypto.KeyGenerator

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class SessionStoreTest {
    private val key by lazy { KeyGenerator.getInstance("AES").apply { init(256) }.generateKey() }
    private fun store(): SessionStore = SessionStore(RuntimeEnvironment.getApplication(), SessionKeyProvider { key })

    @Test fun lastVerifiedViewerSurvivesRestartAndInvalidViewerCannotReplaceIt() {
        val store = store()
        store.clear()
        val server = ServerAddress("https://example.com")
        val viewer = Viewer("Living Room", "server-1", "viewer-1", "Alex")
        store.save(SavedSession(server, "token-123", viewer))
        assertEquals(viewer, store().load()?.viewer)
        for (invalid in listOf(
            viewer.copy(id = ""), viewer.copy(id = "bad id"),
            viewer.copy(serverId = ""), viewer.copy(serverId = "x".repeat(257)),
            viewer.copy(name = "x".repeat(121)), viewer.copy(name = "bad\nname"),
        )) {
            assertThrows(IllegalArgumentException::class.java) {
                store.save(SavedSession(server, "token-123", invalid))
            }
            assertEquals(viewer, store.load()?.viewer)
        }
    }

    @Test fun cachedCatalogIsScopedAndRejectedSnapshotsDoNotReplaceGoodContent() {
        val store = store()
        store.clear()
        val server = ServerAddress("https://example.com")
        val viewer = Viewer("Living Room", "server-1", "viewer-1", "Alex")
        val page = CatalogPage(listOf(CatalogItem("item-1", "video", "Film", "", "", "")), 1, 0, 24)
        store.saveCatalog(server, viewer, "library-all", page)
        assertEquals(page, store().loadCatalog(server, viewer, "library-all"))
        assertEquals(null, store.loadCatalog(server, viewer.copy(id = "other"), "library-all"))
        assertEquals(null, store.loadCatalog(ServerAddress("https://other.example"), viewer, "library-all"))
        assertThrows(IllegalArgumentException::class.java) {
            store.saveCatalog(server, viewer, "library-unknown", page)
        }
        assertThrows(IllegalArgumentException::class.java) {
            store.saveCatalog(server, viewer, "library-all",
                page.copy(items = listOf(page.items.first().copy(title = "x".repeat(513)))))
        }
        assertEquals(page, store.loadCatalog(server, viewer, "library-all"))
        store.clear()
        assertEquals(null, store.loadCatalog(server, viewer, "library-all"))
    }
}
