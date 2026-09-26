package com.kinosail.player.wear

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.os.IBinder
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/** Keeps movie-position anchors current while an opted-in graph is recording. */
class HeartTrackingService : Service() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private var polling: Job? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val timeline = HeartStore.load(this)
        if (timeline?.tracking != true || !hasHeartPermission(this)) {
            stopSelf(startId)
            return START_NOT_STICKY
        }
        val channel = "kinosail_movie_heart"
        getSystemService(NotificationManager::class.java).createNotificationChannel(
            NotificationChannel(channel, "Movie heart graph", NotificationManager.IMPORTANCE_LOW))
        val open = PendingIntent.getActivity(this, 0, Intent(this, WearActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        val notification = Notification.Builder(this, channel)
            .setSmallIcon(android.R.drawable.ic_media_play)
            .setContentTitle("Recording movie heart graph")
            .setContentText(timeline.title)
            .setContentIntent(open).setOngoing(true).build()
        startForeground(1, notification)
        if (polling == null) polling = scope.launch {
            val remote = WearRemoteSession(applicationContext)
            while (isActive && HeartStore.load(this@HeartTrackingService)?.tracking == true &&
                hasHeartPermission(this@HeartTrackingService)) {
                remote.refresh()
                delay(10_000)
            }
            if (HeartStore.load(this@HeartTrackingService)?.tracking == true)
                HeartStore.stop(this@HeartTrackingService, System.currentTimeMillis())
            HeartMonitor.stop(this@HeartTrackingService)
            stopSelf()
        }
        return START_STICKY
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }
}
