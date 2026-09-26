package com.kinosail.player.wear

import android.content.pm.PackageManager
import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.content.ContextCompat
import com.kinosail.player.watchcore.HeartTimeline
import com.kinosail.player.watchcore.WatchPlayer
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch

class WearActivity : ComponentActivity() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private val remote by lazy { WearRemoteSession(applicationContext) }
    private var timeline by mutableStateOf<HeartTimeline?>(null)
    private var heartMessage by mutableStateOf<String?>(null)
    private var pendingPlayer: WatchPlayer? = null

    private val foregroundPermission = registerForActivityResult(ActivityResultContracts.RequestPermission()) { allowed ->
        if (allowed) requestBackgroundOrStart() else heartMessage = "Allow heart rate access to make a graph."
    }
    private val backgroundPermission = registerForActivityResult(ActivityResultContracts.RequestPermission()) { allowed ->
        if (allowed) beginTracking() else heartMessage = "Allow background heart rate access to track a whole movie."
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        timeline = HeartStore.load(this)
        if (timeline?.tracking == true) {
            if (hasHeartPermission(this)) {
                runCatching { ContextCompat.startForegroundService(this, Intent(this, HeartTrackingService::class.java)) }
                    .onFailure {
                        timeline = HeartStore.stop(this, System.currentTimeMillis())
                        scope.launch { HeartMonitor.stop(this@WearActivity) }
                    }
            } else {
                timeline = HeartStore.stop(this, System.currentTimeMillis())
                scope.launch { HeartMonitor.stop(this@WearActivity) }
            }
        }
        setContent {
            WatchApp(remote, timeline, heartMessage,
                onStartHeart = ::requestHeartTracking,
                onStopHeart = ::stopHeartTracking,
                onRefreshHeart = { timeline = HeartStore.load(this) })
        }
    }

    private fun requestHeartTracking(player: WatchPlayer) {
        if (!player.active || player.audio || player.duration <= 0 || timeline?.tracking == true) return
        pendingPlayer = player
        heartMessage = null
        if (ContextCompat.checkSelfPermission(this, foregroundHeartPermission()) == PackageManager.PERMISSION_GRANTED)
            requestBackgroundOrStart()
        else foregroundPermission.launch(foregroundHeartPermission())
    }

    private fun requestBackgroundOrStart() {
        val permission = backgroundHeartPermission()
        if (permission == null || ContextCompat.checkSelfPermission(this, permission) == PackageManager.PERMISSION_GRANTED)
            beginTracking()
        else backgroundPermission.launch(permission)
    }

    private fun beginTracking() {
        val player = pendingPlayer ?: return
        pendingPlayer = null
        scope.launch {
            try {
                if (!HeartMonitor.start(this@WearActivity)) {
                    heartMessage = "Heart rate monitoring is unavailable on this watch."
                    return@launch
                }
                timeline = HeartStore.start(this@WearActivity, player, System.currentTimeMillis())
                ContextCompat.startForegroundService(this@WearActivity,
                    Intent(this@WearActivity, HeartTrackingService::class.java))
                heartMessage = null
            } catch (_: Exception) {
                runCatching { HeartStore.stop(this@WearActivity, System.currentTimeMillis()) }
                HeartMonitor.stop(this@WearActivity)
                heartMessage = "Heart rate monitoring could not start."
            }
        }
    }

    private fun stopHeartTracking() {
        timeline = HeartStore.stop(this, System.currentTimeMillis())
        stopService(Intent(this, HeartTrackingService::class.java))
        scope.launch { HeartMonitor.stop(this@WearActivity) }
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }
}
