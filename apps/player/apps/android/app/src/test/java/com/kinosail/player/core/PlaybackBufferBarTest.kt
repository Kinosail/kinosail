package com.kinosail.player.core

import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import android.view.View
import android.view.ViewGroup
import androidx.media3.common.util.UnstableApi
import androidx.media3.ui.DefaultTimeBar
import androidx.media3.ui.PlayerView
import com.kinosail.player.R
import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.RuntimeEnvironment
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

// Failure cases: the app theme must not hide the stock buffered portion;
// clearing the buffer must remove its highlight without moving playback.
@UnstableApi
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
@GraphicsMode(GraphicsMode.Mode.NATIVE)
class PlaybackBufferBarTest {
    @Test fun stockPlaybackControlsShowAndClearBuffersAtPhoneAndTvWidths() {
        val context = RuntimeEnvironment.getApplication()
        context.setTheme(R.style.Theme_Kinosail)
        val bar = requireNotNull(findBar(PlayerView(context).apply { useController = true }))
        for (width in listOf(320, 1920)) {
            bar.measure(View.MeasureSpec.makeMeasureSpec(width, View.MeasureSpec.EXACTLY),
                View.MeasureSpec.makeMeasureSpec(44, View.MeasureSpec.EXACTLY))
            bar.layout(0, 0, width, 44)
            bar.setDuration(100_000); bar.setPosition(20_000); bar.setBufferedPosition(60_000)
            val loaded = image(bar)
            val buffered = Color.red(loaded.getPixel(width / 2, 22))
            val empty = Color.red(loaded.getPixel(width * 9 / 10, 22))
            assertTrue("Buffered portion should be visibly brighter", buffered - empty > 80)
            assertTrue(bar.isFocusable)
            val folder = File("build/reports/buffer-bar").apply { mkdirs() }
            File(folder, "android-$width.png").outputStream().use { loaded.compress(Bitmap.CompressFormat.PNG, 100, it) }
            bar.setBufferedPosition(0)
            assertEquals(empty, Color.red(image(bar).getPixel(width / 2, 22)))
        }
    }

    private fun image(view: View): Bitmap = Bitmap.createBitmap(view.width, view.height, Bitmap.Config.ARGB_8888).also {
        val canvas = Canvas(it); canvas.drawColor(Color.BLACK); view.draw(canvas)
    }

    private fun findBar(view: View): DefaultTimeBar? {
        if (view is DefaultTimeBar) return view
        if (view is ViewGroup) for (index in 0 until view.childCount) findBar(view.getChildAt(index))?.let { return it }
        return null
    }
}
