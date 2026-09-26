package com.kinosail.player.design

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.tv.material3.MaterialTheme as TvMaterialTheme
import androidx.tv.material3.darkColorScheme as tvDarkColorScheme

object KinoColor {
    val background = Color(0xFF0B0D0B)
    val surface = Color(0xFF151914)
    val raised = Color(0xFF20271E)
    val text = Color(0xFFF6F8F2)
    val muted = Color(0xFFA0A79C)
    val signal = Color(0xFFC4FF47)
    val signalInk = Color(0xFF142000)
}

@Composable
fun KinoTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = darkColorScheme(
        primary = KinoColor.signal,
        onPrimary = KinoColor.signalInk,
        secondaryContainer = KinoColor.raised,
        onSecondaryContainer = KinoColor.text,
        background = KinoColor.background,
        onBackground = KinoColor.text,
        surface = KinoColor.surface,
        surfaceContainerLow = KinoColor.surface,
        onSurface = KinoColor.text,
        surfaceVariant = KinoColor.raised,
        onSurfaceVariant = KinoColor.muted,
        outline = KinoColor.muted,
        outlineVariant = KinoColor.raised,
    ), content = content)
}

@Composable
fun TvKinoTheme(content: @Composable () -> Unit) {
    TvMaterialTheme(
        colorScheme = tvDarkColorScheme(
            primary = KinoColor.signal,
            onPrimary = KinoColor.signalInk,
            background = KinoColor.background,
            onBackground = KinoColor.text,
            surface = KinoColor.surface,
            onSurface = KinoColor.text,
            surfaceVariant = KinoColor.raised,
            onSurfaceVariant = KinoColor.muted,
        ),
        content = content,
    )
}
