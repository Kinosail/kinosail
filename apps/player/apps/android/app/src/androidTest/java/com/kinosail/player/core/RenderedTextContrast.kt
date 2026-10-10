package com.kinosail.player.core

import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.captureToImage
import androidx.core.graphics.ColorUtils
import org.junit.Assert.assertTrue

// Compose accessibility checks missed black headings on the dark library background.
// Check rendered pixels at the journey boundary instead of asserting theme tokens.
internal fun assertReadableHeading(node: SemanticsNodeInteraction) {
    val image = node.captureToImage().asAndroidBitmap()
    val pixels = IntArray(image.width * image.height)
    image.getPixels(pixels, 0, image.width, 0, 0, image.width, image.height)
    val colors = pixels.asSequence().groupingBy { it or (0xff shl 24) }.eachCount()
        .entries.sortedByDescending { it.value }
    val background = colors.first().key
    val contrast = colors.drop(1).filter { it.value >= 10 }
        .maxOf { ColorUtils.calculateContrast(it.key, background) }
    assertTrue("Large library heading contrast was $contrast; expected at least 3:1", contrast >= 3.0)
}
