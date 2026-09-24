package com.kinosail.player.tv

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.kinosail.player.R
import com.kinosail.player.design.SailBackdrop
import com.kinosail.player.design.TvKinoTheme
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Text

class TvActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent { TvKinoTheme { TvStart() } }
    }
}

@Composable
private fun TvStart() {
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        SailBackdrop()
        Column(
            Modifier.fillMaxSize().safeDrawingPadding().padding(64.dp),
            verticalArrangement = Arrangement.SpaceBetween,
        ) {
            Text(
                stringResource(R.string.app_name),
                style = MaterialTheme.typography.headlineLarge,
                color = MaterialTheme.colorScheme.onBackground,
            )
            Column(verticalArrangement = Arrangement.spacedBy(20.dp)) {
                Text(
                    stringResource(R.string.tv_start_title),
                    style = MaterialTheme.typography.displayMedium,
                    color = MaterialTheme.colorScheme.onBackground,
                )
                Text(
                    stringResource(R.string.tv_start_body),
                    style = MaterialTheme.typography.titleLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}
