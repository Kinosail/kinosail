package com.kinosail.player.design

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
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
    val lightBackground = Color(0xFFF4F8EF)
    val lightSurface = Color.White
    val lightRaised = Color(0xFFE9EFE2)
    val lightText = Color(0xFF162011)
    val lightMuted = Color(0xFF5B6852)
    val lightSignal = Color(0xFF3C6100)
}

@Composable
fun KinoTheme(content: @Composable () -> Unit) {
    val colors = if (isSystemInDarkTheme()) {
        darkColorScheme(
            primary = KinoColor.signal,
            onPrimary = KinoColor.signalInk,
            background = KinoColor.background,
            onBackground = KinoColor.text,
            surface = KinoColor.surface,
            surfaceContainerLow = KinoColor.surface,
            onSurface = KinoColor.text,
            surfaceVariant = KinoColor.raised,
            onSurfaceVariant = KinoColor.muted,
        )
    } else {
        lightColorScheme(
            primary = KinoColor.lightSignal,
            onPrimary = Color.White,
            background = KinoColor.lightBackground,
            onBackground = KinoColor.lightText,
            surface = KinoColor.lightSurface,
            surfaceContainerLow = KinoColor.lightSurface,
            onSurface = KinoColor.lightText,
            surfaceVariant = KinoColor.lightRaised,
            onSurfaceVariant = KinoColor.lightMuted,
        )
    }
    MaterialTheme(colorScheme = colors, content = content)
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
