package com.kinosail.player.wear

import android.Manifest
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.os.SystemClock
import androidx.core.content.ContextCompat
import androidx.health.services.client.HealthServices
import androidx.health.services.client.PassiveListenerService
import androidx.health.services.client.data.DataPointContainer
import androidx.health.services.client.data.DataType
import androidx.health.services.client.data.PassiveListenerConfig
import java.time.Instant
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

internal fun foregroundHeartPermission(): String = if (Build.VERSION.SDK_INT >= 36)
    "android.permission.health.READ_HEART_RATE" else Manifest.permission.BODY_SENSORS

internal fun backgroundHeartPermission(): String? = when {
    Build.VERSION.SDK_INT >= 36 -> "android.permission.health.READ_HEALTH_DATA_IN_BACKGROUND"
    Build.VERSION.SDK_INT >= 33 -> Manifest.permission.BODY_SENSORS_BACKGROUND
    else -> null
}

internal fun hasHeartPermission(context: Context): Boolean =
    ContextCompat.checkSelfPermission(context, foregroundHeartPermission()) == PackageManager.PERMISSION_GRANTED &&
        (backgroundHeartPermission()?.let {
            ContextCompat.checkSelfPermission(context, it) == PackageManager.PERMISSION_GRANTED
        } ?: true)

internal object HeartMonitor {
    suspend fun start(context: Context): Boolean = withContext(Dispatchers.IO) {
        if (!hasHeartPermission(context)) return@withContext false
        val passive = HealthServices.getClient(context).passiveMonitoringClient
        val capabilities = passive.getCapabilitiesAsync().get(10, TimeUnit.SECONDS)
        if (DataType.HEART_RATE_BPM !in capabilities.supportedDataTypesPassiveMonitoring) return@withContext false
        val config = PassiveListenerConfig.builder().setDataTypes(setOf(DataType.HEART_RATE_BPM)).build()
        passive.setPassiveListenerServiceAsync(MovieHeartListener::class.java, config).get(10, TimeUnit.SECONDS)
        true
    }

    suspend fun stop(context: Context) = withContext(Dispatchers.IO) {
        runCatching { HealthServices.getClient(context).passiveMonitoringClient
            .clearPassiveListenerServiceAsync().get(10, TimeUnit.SECONDS) }
        Unit
    }
}

class MovieHeartListener : PassiveListenerService() {
    override fun onNewDataPointsReceived(dataPoints: DataPointContainer) {
        if (!hasHeartPermission(this)) { HeartStore.stop(this, System.currentTimeMillis()); return }
        val bootInstant = Instant.now().minusMillis(SystemClock.elapsedRealtime())
        val nowMs = System.currentTimeMillis()
        if (HeartStore.load(this, nowMs)?.tracking != true) {
            HealthServices.getClient(this).passiveMonitoringClient.clearPassiveListenerServiceAsync()
            return
        }
        for (sample in dataPoints.getData(DataType.HEART_RATE_BPM)) {
            val atMs = sample.getTimeInstant(bootInstant).toEpochMilli()
            HeartStore.reading(this, atMs, sample.value, nowMs)
        }
    }

    override fun onPermissionLost() {
        HeartStore.stop(this, System.currentTimeMillis())
        HealthServices.getClient(this).passiveMonitoringClient.clearPassiveListenerServiceAsync()
    }
}
