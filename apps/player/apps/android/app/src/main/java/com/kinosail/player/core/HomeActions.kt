package com.kinosail.player.core

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.FilledTonalIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.kinosail.player.R
import com.kinosail.player.design.KinoColor

internal val CatalogItem.homeSubtitleItem get() = copy(plot = "", year = if (kind == "video" || kind == "show") "" else year)

@Composable
internal fun PhoneHomeHeader(listening: Boolean, browse: (String) -> Unit) {
    Box(Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
        Text("Kinosail", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold, color = KinoColor.text,
            modifier = Modifier.align(Alignment.Center).semantics { heading() })
        FilledTonalButton(onClick = { browse(if (listening) "home" else "listen") }, modifier = Modifier.align(Alignment.CenterEnd),
            colors = ButtonDefaults.filledTonalButtonColors(containerColor = KinoColor.raised, contentColor = KinoColor.signal)) {
            Text(interfaceText(if (listening) "Watch" else "Listen"))
        }
    }
}

@Composable
internal fun HomeFeatureActions(item: CatalogItem, myList: (() -> Unit)?, play: () -> Unit, details: () -> Unit) {
    FlowRow(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Button(onClick = play) {
            if (item.kind == "book") Icon(painterResource(R.drawable.nav_books), null, Modifier.size(18.dp))
            else Icon(Icons.Filled.PlayArrow, null, Modifier.size(20.dp))
            Spacer(Modifier.width(8.dp))
            Text(interfaceText(item.playLabel))
        }
        FilledTonalButton(onClick = details,
            colors = ButtonDefaults.filledTonalButtonColors(containerColor = KinoColor.raised, contentColor = KinoColor.text)) {
            Text(interfaceText("Details"))
        }
        if (myList != null) FilledTonalIconButton(onClick = myList,
            colors = IconButtonDefaults.filledTonalIconButtonColors(containerColor = KinoColor.raised, contentColor = KinoColor.text)) {
            Icon(painterResource(R.drawable.nav_list), interfaceText("My List"), Modifier.size(20.dp))
        }
    }
}
