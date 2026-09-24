package com.kinosail.player.mobile

import android.app.PictureInPictureParams
import android.content.pm.PackageManager
import android.content.Intent
import android.content.res.Configuration
import android.graphics.Rect
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.util.Rational
import android.view.View
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.kinosail.player.R
import com.kinosail.player.core.ConnectionModel
import com.kinosail.player.core.ConnectionPhase
import com.kinosail.player.core.VideoPipHost
import com.kinosail.player.design.KinoTheme
import com.kinosail.player.design.SailBackdrop

class MobileActivity : ComponentActivity(), VideoPipHost {
    override var inPictureInPicture by mutableStateOf(false)
        private set
    private var videoPipReady = false
    private var videoPipView: View? = null
    private val pipLayoutListener = View.OnLayoutChangeListener { _, _, _, _, _, _, _, _, _ -> updatePipParams() }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent { KinoTheme { MobileStart() } }
    }

    override fun setVideoPipReady(ready: Boolean) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O ||
            !packageManager.hasSystemFeature(PackageManager.FEATURE_PICTURE_IN_PICTURE)) {
            videoPipReady = false
            return
        }
        videoPipReady = ready
        updatePipParams()
    }

    override fun setVideoPipView(view: View?) {
        if (videoPipView === view) return
        videoPipView?.removeOnLayoutChangeListener(pipLayoutListener)
        videoPipView = view
        view?.addOnLayoutChangeListener(pipLayoutListener)
        updatePipParams()
    }

    private fun updatePipParams() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O ||
            !packageManager.hasSystemFeature(PackageManager.FEATURE_PICTURE_IN_PICTURE)) return
        setPictureInPictureParams(pipParams())
    }

    @androidx.annotation.RequiresApi(Build.VERSION_CODES.O)
    private fun pipParams(): PictureInPictureParams {
        val params = PictureInPictureParams.Builder().setAspectRatio(Rational(16, 9))
        videoPipView?.let { view ->
            val bounds = Rect()
            if (view.getGlobalVisibleRect(bounds) && !bounds.isEmpty) params.setSourceRectHint(bounds)
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S)
            params.setAutoEnterEnabled(videoPipReady)
        return params.build()
    }

    override fun onUserLeaveHint() {
        super.onUserLeaveHint()
        if (videoPipReady && Build.VERSION.SDK_INT in Build.VERSION_CODES.O until Build.VERSION_CODES.S)
            enterPictureInPictureMode(pipParams())
    }

    override fun onPictureInPictureModeChanged(isInPictureInPictureMode: Boolean, newConfig: Configuration) {
        super.onPictureInPictureModeChanged(isInPictureInPictureMode, newConfig)
        inPictureInPicture = isInPictureInPictureMode
    }
}

@Composable
private fun MobileStart() {
    val connection: ConnectionModel = viewModel()
    val phase = connection.phase
    val context = LocalContext.current
    if (phase is ConnectionPhase.Connected) {
        MobileLibrary(connection, phase.viewer)
        return
    }
    BackHandler(phase is ConnectionPhase.Pairing) { connection.cancelPairing() }
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        if (isSystemInDarkTheme()) SailBackdrop()
        Column(Modifier.fillMaxSize().safeDrawingPadding().padding(24.dp), verticalArrangement = Arrangement.SpaceBetween) {
            Text("Kinosail", style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground)
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                when (phase) {
                    ConnectionPhase.Restoring -> {
                        Text("Restoring your Server", style = MaterialTheme.typography.displaySmall,
                            color = MaterialTheme.colorScheme.onBackground)
                        Text("Checking your saved connection…", color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    ConnectionPhase.Setup -> {
                        Text("Your library starts here", style = MaterialTheme.typography.displaySmall,
                            color = MaterialTheme.colorScheme.onBackground)
                        Text("Enter your Kinosail Server address to connect this device.",
                            style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        OutlinedTextField(value = connection.address,
                            onValueChange = { if (it.length <= 2048) connection.address = it },
                            label = { Text("Server address") }, placeholder = { Text("https://your-server") },
                            singleLine = true, enabled = !connection.busy,
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Go),
                            keyboardActions = KeyboardActions(onGo = { connection.connect("Kinosail Android phone") }),
                            modifier = Modifier.fillMaxWidth())
                        Button(onClick = { connection.connect("Kinosail Android phone") },
                            enabled = !connection.busy, modifier = Modifier.fillMaxWidth()) {
                            Text(if (connection.busy) "Connecting…" else "Connect")
                        }
                    }
                    is ConnectionPhase.Pairing -> {
                        Text("Approve this device", style = MaterialTheme.typography.displaySmall,
                            color = MaterialTheme.colorScheme.onBackground)
                        Text("On a device signed in to ${phase.server}, open Quick Connect and enter this code.",
                            style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Text(phase.code, style = MaterialTheme.typography.displayLarge,
                            fontFamily = FontFamily.Monospace, letterSpacing = 4.sp,
                            color = MaterialTheme.colorScheme.primary)
                        Text("Waiting for approval…", color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Button(onClick = {
                            val uri = Uri.parse("${phase.server}/quick-connect?code=${phase.code}")
                            context.startActivity(Intent(Intent.ACTION_VIEW, uri))
                        }, modifier = Modifier.fillMaxWidth()) { Text("Open approval page") }
                        TextButton(onClick = connection::cancelPairing, modifier = Modifier.fillMaxWidth()) {
                            Text("Cancel")
                        }
                    }
                    is ConnectionPhase.Connected -> Unit
                }
                connection.notice?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            }
        }
    }
}
