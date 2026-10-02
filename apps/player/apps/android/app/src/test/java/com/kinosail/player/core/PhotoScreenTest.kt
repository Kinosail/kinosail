package com.kinosail.player.core

import android.app.Application
import android.graphics.Bitmap
import android.graphics.Color
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsFocused
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performKeyInput
import androidx.compose.ui.test.pressKey
import androidx.compose.ui.input.key.Key as ComposeKey
import androidx.compose.ui.test.printToString
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.lifecycle.ViewModelStore
import androidx.test.core.app.ApplicationProvider
import com.kinosail.player.design.KinoTheme
import com.kinosail.player.design.TvKinoTheme
import com.sun.net.httpserver.HttpServer
import java.io.ByteArrayOutputStream
import java.io.File
import java.io.InputStream
import java.io.OutputStream
import java.net.InetSocketAddress
import java.security.Key
import java.security.KeyStoreSpi
import java.security.Provider
import java.security.Security
import java.security.cert.Certificate
import java.util.Collections
import java.util.Date
import java.util.concurrent.Executors
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import javax.crypto.KeyGenerator
import org.junit.After
import org.junit.Assert.*
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xhdpi")
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class PhotoScreenTest {
    @get:Rule val compose = createComposeRule()
    private val application = ApplicationProvider.getApplicationContext<Application>()
    private val models = ViewModelStore()
    private lateinit var server: HttpServer
    private val executor = Executors.newCachedThreadPool { task -> Thread(task).apply { isDaemon = true } }
    @Volatile private var photoStatus = 200
    @Volatile private var photoGate = CountDownLatch(0)
    private val photoRequests = AtomicInteger()
    @Volatile private var permitted = true
    @Volatile private var photoTitle = "A light photo"

    @Before fun preparePublicClientSession() {
        assertNull(Security.getProvider("AndroidKeyStore"))
        Security.addProvider(object : Provider("AndroidKeyStore", 1.0, "JVM session key fixture") {
            init { put("KeyStore.AndroidKeyStore", FixtureKeyStore::class.java.name) }
        })
        SessionStore(application).clear()
        server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0).apply { executor = this@PhotoScreenTest.executor }
        val metrics = application.resources.displayMetrics
        val image = Bitmap.createBitmap(metrics.widthPixels, metrics.heightPixels, Bitmap.Config.ARGB_8888)
            .apply { eraseColor(Color.WHITE) }
        val photo = ByteArrayOutputStream().also { image.compress(Bitmap.CompressFormat.PNG, 100, it) }.toByteArray()
        server.createContext("/") { request ->
            val authorized = request.requestHeaders.getFirst("Authorization") == "Bearer photo-test" &&
                request.requestHeaders.getFirst("X-Kinosail-Viewer-Profile") == "photo-viewer"
            val media = request.requestURI.path == "/media/light-photo"
            if (media) { photoRequests.incrementAndGet(); check(photoGate.await(15, TimeUnit.SECONDS)) }
            val body = if (media) photo else
                """{"items":[{"id":"light-photo","kind":"photo","title":"$photoTitle","stream":"${if (permitted) "/media/light-photo" else ""}"}],"total":1,"offset":0,"limit":24}""".toByteArray()
            request.responseHeaders.set("Content-Type", if (media) "image/png" else "application/json")
            request.sendResponseHeaders(if (!authorized) 401 else if (media) photoStatus else 200, body.size.toLong())
            request.responseBody.use { it.write(body) }
        }
        server.start()
        val viewer = Viewer("Photo fixture", "photo-fixture", "photo-viewer", "Photo Viewer")
        SessionStore(application).save(SavedSession(ServerAddress("http://127.0.0.1:${server.address.port}"), "photo-test", viewer))
    }

    @After fun releaseFixture() {
        photoGate.countDown()
        models.clear()
        if (::server.isInitialized) server.stop(0)
        executor.shutdownNow()
        SessionStore(application).clear()
        Security.removeProvider("AndroidKeyStore")
    }

    @Test fun titleRemainsReadableOverLoadedLightPhoto() = verifyLightPhoto("phone")

    @Test @Config(qualifiers = "w834dp-h1210dp-mdpi")
    fun tabletPhotoTitleRemainsReadable() = verifyLightPhoto("tablet")

    @Test @Config(qualifiers = "w844dp-h390dp-mdpi")
    fun landscapePhotoTitleRemainsReadable() = verifyLightPhoto("landscape")

    @Test @Config(qualifiers = "w1210dp-h834dp-mdpi")
    fun landscapeTabletPhotoTitleRemainsReadable() = verifyLightPhoto("tablet-landscape")

    @Test @Config(qualifiers = "w960dp-h540dp-mdpi")
    fun tvPhotoTitleAndRemoteFocusRemainAvailable() = verifyLightPhoto("tv", tv = true)

    @Test fun largerTextAndLongPhotoTitleRemainReadable() {
        RuntimeEnvironment.setFontScale(1.3f)
        photoTitle = "A light photo from a long afternoon beside the sea"
        verifyLightPhoto("phone-large-text")
    }

    @Test fun failedFetchRetriesWithTruthfulPendingFeedback() {
        photoStatus = 503
        photoGate = CountDownLatch(1)
        var closed = 0
        openPhoto(close = { closed++ })
        waitForText("Opening photo…")
        compose.onNodeWithText("Zoom in").assertIsNotEnabled()
        capture("phone-pending")
        photoGate.countDown()
        val error = "Could not open the photo. Check your Server connection and try again."
        waitForText(error)
        compose.onNodeWithText(error).assert(SemanticsMatcher.expectValue(SemanticsProperties.LiveRegion, LiveRegionMode.Polite))
        compose.onNodeWithText("Opening photo…").assertDoesNotExist()
        compose.onNodeWithText("Zoom in").assertIsNotEnabled()
        capture("phone-failed")
        photoStatus = 200
        photoGate = CountDownLatch(1)
        compose.onNodeWithText("Try again").performClick()
        waitForText("Opening photo…")
        compose.onNodeWithText(error).assertDoesNotExist()
        compose.onNodeWithText("Try again").assertDoesNotExist()
        capture("phone-retry-pending")
        photoGate.countDown()
        waitForLoaded()
        capture("phone-recovered")
        assertEquals(2, photoRequests.get())
        compose.onNodeWithText("Done").performClick()
        assertEquals(1, closed)
    }

    @Test fun viewerWithoutPhotoPermissionHasNoRetryOrMediaRequest() {
        permitted = false
        openPhoto()
        waitForText("This Viewer cannot open the photo.")
        compose.onNodeWithText("Opening photo…").assertDoesNotExist()
        compose.onNodeWithText("Try again").assertDoesNotExist()
        compose.onNodeWithText("Zoom in").assertIsNotEnabled()
        capture("phone-permission-denied")
        assertEquals(0, photoRequests.get())
    }

    private fun openPhoto(tv: Boolean = false, close: () -> Unit = {}) {
        val catalog = CatalogModel(application).also { models.put("photo", it) }
        catalog.open(Viewer("Photo fixture", "photo-fixture", "photo-viewer", "Photo Viewer"))
        compose.setContent {
            val content: @androidx.compose.runtime.Composable () -> Unit = {
                catalog.state.items.firstOrNull()?.let { PhotoScreen(it, catalog, tv = tv, close = close) }
            }
            if (tv) TvKinoTheme(content) else KinoTheme(content)
        }
    }

    private fun waitForText(text: String) {
        compose.waitUntil(15_000) { compose.onAllNodesWithText(text).fetchSemanticsNodes().isNotEmpty() }
        compose.onNodeWithText(text).assertIsDisplayed()
    }

    private fun waitForLoaded() {
        compose.waitUntil(15_000) { compose.onAllNodesWithText("Zoom in").fetchSemanticsNodes().any { node ->
            !node.config.contains(SemanticsProperties.Disabled)
        } }
        compose.onNodeWithText("Zoom in").assertIsEnabled()
        compose.onNodeWithText("Opening photo…").assertDoesNotExist()
    }

    private fun verifyLightPhoto(name: String, tv: Boolean = false) {
        openPhoto(tv)
        waitForLoaded()
        if (tv) compose.onNodeWithText("Zoom in").assertIsFocused()
        val title = compose.onNodeWithText(photoTitle).assertIsDisplayed().fetchSemanticsNode().boundsInRoot
        val pixels = capture("$name-loaded")
        for (text in listOf(photoTitle, "Zoom in", "Done")) {
            val bounds = compose.onNodeWithText(text).fetchSemanticsNode().boundsInRoot
            assertTrue("$text must fit the viewport", bounds.left >= 0 && bounds.top >= 0 &&
                bounds.right <= pixels.width && bounds.bottom <= pixels.height)
        }
        assertEquals("The real photo must be loaded below the overlay", Color.WHITE, pixels.getPixel(pixels.width / 2, pixels.height / 2))
        val background = Color.luminance(pixels.getPixel(title.left.toInt() + 1, title.top.toInt() + 1))
        val contrast = 1.05 / (background + .05)
        assertTrue("White photo title contrast was $contrast", contrast >= 4.5)
        if (tv) compose.onNodeWithText("Zoom in").performKeyInput { pressKey(ComposeKey.DirectionCenter) }
        else compose.onNodeWithText("Zoom in").performClick()
        compose.onNodeWithText("Fit photo").assertIsEnabled()
        capture("$name-zoomed")
        if (tv) compose.onNodeWithContentDescription(photoTitle).assertIsFocused()
            .performKeyInput { pressKey(ComposeKey.DirectionCenter) }
        else compose.onNodeWithText("Fit photo").performClick()
        compose.onNodeWithText("Zoom in").assertIsEnabled()
        assertEquals(1, photoRequests.get())
    }

    private fun capture(name: String): Bitmap {
        val root = compose.onRoot()
        val pixels = root.captureToImage().asAndroidBitmap()
        val output = File("build/reports/photo-screen/$name.png").apply { parentFile?.mkdirs() }
        output.outputStream().use { pixels.compress(Bitmap.CompressFormat.PNG, 100, it) }
        File(output.parentFile, "$name-semantics.txt").writeText(root.printToString())
        return pixels
    }

    // Supply only the platform key lookup missing on the JVM. SessionStore still encrypts,
    // saves and reloads the public session; CatalogModel and ArtworkClient use real HTTP.
    class FixtureKeyStore : KeyStoreSpi() {
        override fun engineGetKey(alias: String?, password: CharArray?): Key = key
        override fun engineLoad(stream: InputStream?, password: CharArray?) {}
        override fun engineContainsAlias(alias: String?) = true
        override fun engineAliases() = Collections.enumeration(listOf("kinosail_android_session_v1"))
        override fun engineSize() = 1
        override fun engineIsKeyEntry(alias: String?) = true
        override fun engineIsCertificateEntry(alias: String?) = false
        override fun engineGetCreationDate(alias: String?) = Date(0)
        override fun engineGetCertificate(alias: String?): Certificate? = null
        override fun engineGetCertificateChain(alias: String?): Array<Certificate>? = null
        override fun engineGetCertificateAlias(certificate: Certificate?) = null
        override fun engineSetKeyEntry(alias: String?, key: Key?, password: CharArray?, chain: Array<Certificate>?) = error("Not used")
        override fun engineSetKeyEntry(alias: String?, key: ByteArray?, chain: Array<Certificate>?) = error("Not used")
        override fun engineSetCertificateEntry(alias: String?, certificate: Certificate?) = error("Not used")
        override fun engineDeleteEntry(alias: String?) = error("Not used")
        override fun engineStore(stream: OutputStream?, password: CharArray?) = error("Not used")
        companion object { private val key = KeyGenerator.getInstance("AES").apply { init(256) }.generateKey() }
    }
}
