package com.kinosail.player.core

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
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
internal fun LibraryLoading(home: Boolean = false, tv: Boolean = false) {
    Column(Modifier.fillMaxWidth().clearAndSetSemantics { contentDescription = "Loading library" },
        verticalArrangement = Arrangement.spacedBy(16.dp)) {
        if (home && !tv) BoxWithConstraints {
            val artworkWidth = maxWidth * 0.46f
            if (maxWidth >= 600.dp) Row(horizontalArrangement = Arrangement.spacedBy(32.dp)) {
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
        Row(horizontalArrangement = Arrangement.spacedBy(18.dp)) {
            repeat(if (tv) 4 else 3) {
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Placeholder(Modifier.fillMaxWidth().aspectRatio(if (tv && home) 16f / 9f else 2f / 3f))
                    Placeholder(Modifier.fillMaxWidth().height(18.dp))
                }
            }
        }
    }
}

@Composable
private fun Placeholder(modifier: Modifier) {
    Box(modifier.clip(RoundedCornerShape(12.dp)).background(KinoColor.raised))
}
