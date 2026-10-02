package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.unit.dp
import com.kinosail.player.design.KinoColor

@Composable
internal fun LibraryLoading(home: Boolean = false, tv: Boolean = false, show: Boolean = false, view: String = "") {
    Column(Modifier.fillMaxWidth().clearAndSetSemantics { contentDescription = "Loading library" },
        verticalArrangement = Arrangement.spacedBy(16.dp)) {
        if (home && !show) Placeholder(Modifier.width(180.dp).height(28.dp))
        if (home && (!tv || show)) BoxWithConstraints {
            val artworkWidth = maxWidth * 0.46f
            if (maxWidth >= 600.dp && androidx.compose.ui.platform.LocalDensity.current.fontScale < 1.5f) Row(horizontalArrangement = Arrangement.spacedBy(32.dp)) {
                Placeholder(Modifier.width(artworkWidth).aspectRatio(16f / 9f))
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                    Placeholder(Modifier.fillMaxWidth().height(40.dp))
                    Placeholder(Modifier.fillMaxWidth(0.7f).height(20.dp))
                    Placeholder(Modifier.width(120.dp).height(48.dp))
                }
            } else Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Placeholder(Modifier.fillMaxWidth().aspectRatio(16f / 9f))
                Placeholder(Modifier.fillMaxWidth(0.8f).height(40.dp))
                Placeholder(Modifier.width(120.dp).height(48.dp))
            }
        }
        if (show && tv) Row(horizontalArrangement = Arrangement.spacedBy(24.dp)) {
            Column(Modifier.width(180.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Placeholder(Modifier.fillMaxWidth().height(28.dp))
                Placeholder(Modifier.fillMaxWidth().height(48.dp))
            }
            LazyRow(horizontalArrangement = Arrangement.spacedBy(18.dp), contentPadding = PaddingValues(12.dp)) {
                items(3) { LoadingCard(300.dp, 16f / 9f) }
            }
        } else if (show) BoxWithConstraints {
            val width = maxWidth
            Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Placeholder(Modifier.width(180.dp).height(28.dp))
                Placeholder(Modifier.width(240.dp).height(48.dp))
                repeat(2) {
                    if (width < 320.dp || androidx.compose.ui.platform.LocalDensity.current.fontScale >= 1.5f)
                        LoadingCard(width, 16f / 9f)
                    else Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        Placeholder(Modifier.width(128.dp).aspectRatio(16f / 9f))
                        Placeholder(Modifier.weight(1f).height(48.dp))
                    }
                }
            }
        } else if (home) LazyRow(horizontalArrangement = Arrangement.spacedBy(18.dp), contentPadding = PaddingValues(12.dp)) {
            items(4) { LoadingCard(if (tv) 320.dp else 144.dp, if (tv) 16f / 9f else 2f / 3f) }
        } else BoxWithConstraints(Modifier.padding(if (tv) 12.dp else 0.dp)) {
            val spacing = if (tv) 20.dp else 18.dp
            val minimum = if (view == "photos") if (tv) 280.dp else 240.dp else if (tv) 160.dp else 144.dp
            val columns = ((maxWidth + spacing) / (minimum + spacing)).toInt().coerceAtLeast(1)
            val width = (maxWidth - spacing * (columns - 1)) / columns
            val ratio = if (view in setOf("music", "audiobooks")) 1f else if (view == "photos") 16f / 9f else 2f / 3f
            Row(horizontalArrangement = Arrangement.spacedBy(spacing)) {
                repeat(columns) { LoadingCard(width, ratio) }
            }
        }
    }
}

@Composable
private fun LoadingCard(width: androidx.compose.ui.unit.Dp, ratio: Float) {
    Column(Modifier.width(width), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Placeholder(Modifier.fillMaxWidth().aspectRatio(ratio))
        Placeholder(Modifier.fillMaxWidth().height(18.dp))
    }
}

@Composable
private fun Placeholder(modifier: Modifier) {
    Box(modifier.clip(RoundedCornerShape(12.dp)).background(KinoColor.raised))
}
