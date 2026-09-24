package com.kinosail.player.design

import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import com.kinosail.player.R

@Composable
fun SailBackdrop() {
    Image(
        painter = painterResource(R.drawable.cinema_sail),
        contentDescription = null,
        modifier = Modifier.fillMaxSize().alpha(0.45f),
        contentScale = ContentScale.Crop,
    )
}

