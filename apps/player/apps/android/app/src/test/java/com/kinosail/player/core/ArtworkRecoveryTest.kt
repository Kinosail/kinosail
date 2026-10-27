package com.kinosail.player.core

import android.app.Application
import android.graphics.Bitmap
import android.graphics.Color
import android.os.Looper
import androidx.compose.foundation.layout.width
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModelStore
import androidx.test.core.app.ApplicationProvider
import com.kinosail.player.design.KinoTheme
import com.sun.net.httpserver.HttpServer
import java.io.ByteArrayOutputStream
import java.io.File
import java.net.InetSocketAddress
import java.security.Provider
import java.security.Security
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicInteger
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class ArtworkRecoveryTest {
    @get:Rule val compose = createComposeRule()
    private val application = ApplicationProvider.getApplicationContext<Application>()
    private val models = ViewModelStore()
    private val viewer = Viewer("Artwork fixture", "artwork-fixture", "artwork-viewer", "Artwork Viewer")
    private lateinit var server: HttpServer
    private val executor = Executors.newCachedThreadPool { task -> Thread(task).apply { isDaemon = true } }
    private val posterRequests = AtomicInteger()
    @Volatile private var outage = true

    @Before fun startFixture() {
        assertNull(Security.getProvider("AndroidKeyStore"))
        Security.addProvider(object : Provider("AndroidKeyStore", 1.0, "JVM session key fixture") {
            init { put("KeyStore.AndroidKeyStore", PhotoScreenTest.FixtureKeyStore::class.java.name) }
        })
        SessionStore(application).clear()
        val poster = ByteArrayOutputStream().also { output ->
            Bitmap.createBitmap(200, 300, Bitmap.Config.ARGB_8888).apply { eraseColor(Color.RED) }
                .compress(Bitmap.CompressFormat.PNG, 100, output)
        }.toByteArray()
        server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0).apply { executor = this@ArtworkRecoveryTest.executor }
        server.createContext("/") { request ->
            val art = request.requestURI.path == "/art/movie"
            if (art) posterRequests.incrementAndGet()
            val failed = art && outage
            val body = if (art && !failed) poster else if (failed) """{"error":"outage"}""".toByteArray() else
                """{"items":[{"id":"movie","kind":"video","title":"Recovery Movie","artwork":"/art/movie"}],"total":1,"offset":0,"limit":24}""".toByteArray()
            request.responseHeaders.set("Content-Type", if (art && !failed) "image/png" else "application/json")
            request.sendResponseHeaders(if (failed) 500 else 200, body.size.toLong())
            request.responseBody.use { it.write(body) }
        }
        server.start()
        SessionStore(application).save(SavedSession(ServerAddress("http://127.0.0.1:${server.address.port}"), "artwork-test", viewer))
    }

    @After fun stopFixture() {
        models.clear()
        if (::server.isInitialized) server.stop(0)
        executor.shutdownNow()
        SessionStore(application).clear()
        Security.removeProvider("AndroidKeyStore")
    }

    @Test fun posterReturnsAfterOutageOutlastingFirstRetries() {
        val catalog = CatalogModel(application).also { models.put("artwork", it) }
        catalog.open(viewer)
        val deadline = System.nanoTime() + 15_000_000_000L
        while (catalog.state.items.isEmpty() && System.nanoTime() < deadline) {
            shadowOf(Looper.getMainLooper()).idle()
            Thread.sleep(10)
        }
        assertEquals("Recovery Movie", catalog.state.items.single().title)
        compose.setContent {
            KinoTheme { catalog.state.items.firstOrNull()?.let { MediaArtwork(it, catalog, Modifier.width(200.dp)) } }
        }
        advanceUntil(seconds = 60) { posterRequests.get() >= 5 }
        assertEquals("The fixture must reject the first five poster loads", 5, posterRequests.get())
        assertNotEquals(Color.RED, capture("artwork-outage").getPixel(40, 60))
        outage = false
        val recovered = advanceUntil(seconds = 120) { capture("artwork-polling").getPixel(40, 60) == Color.RED }
        capture("artwork-recovered")
        println("ARTWORK_RECOVERY posterRequests=${posterRequests.get()} recovered=$recovered")
        assertTrue("The poster must return after the outage while the card stays on screen", recovered)
    }

    private fun advanceUntil(seconds: Int, condition: () -> Boolean): Boolean {
        repeat(seconds) {
            if (condition()) return true
            compose.mainClock.advanceTimeBy(1_000)
            Thread.sleep(60)
            shadowOf(Looper.getMainLooper()).idle()
            compose.waitForIdle()
        }
        return condition()
    }

    private fun capture(name: String): Bitmap {
        val pixels = compose.onRoot().captureToImage().asAndroidBitmap()
        val output = File("build/reports/artwork-recovery/$name.png").apply { parentFile?.mkdirs() }
        output.outputStream().use { pixels.compress(Bitmap.CompressFormat.PNG, 100, it) }
        return pixels
    }
}
