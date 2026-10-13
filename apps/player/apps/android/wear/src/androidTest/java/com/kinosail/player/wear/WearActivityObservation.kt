package com.kinosail.player.wear

import android.app.Activity
import android.app.KeyguardManager
import android.os.Handler
import android.os.Looper
import android.os.PowerManager
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.runner.lifecycle.ActivityLifecycleMonitorRegistry
import androidx.test.runner.lifecycle.Stage
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference

// Temporary diagnosis for Activities displaced by the system during hosted Wear journeys.
internal fun wearActivityObservation(activityType: Class<out Activity>): String {
    val observation = AtomicReference("unavailable")
    val done = CountDownLatch(1)
    Handler(Looper.getMainLooper()).post {
        try {
            val context = InstrumentationRegistry.getInstrumentation().targetContext
            val power = requireNotNull(context.getSystemService(PowerManager::class.java))
            val keyguard = requireNotNull(context.getSystemService(KeyguardManager::class.java))
            val monitor = ActivityLifecycleMonitorRegistry.getInstance()
            val rows = Stage.values().flatMap { lifecycle ->
                monitor.getActivitiesInStage(lifecycle).filter { it.javaClass == activityType }.take(2).map {
                    "${lifecycle.name}:focus=${it.hasWindowFocus()},finishing=${it.isFinishing}," +
                        "destroyed=${it.isDestroyed},changing=${it.isChangingConfigurations}"
                }
            }
            observation.set(rows.joinToString("|").ifEmpty { "none" } +
                ";interactive=${power.isInteractive},keyguardLocked=${keyguard.isKeyguardLocked}," +
                "deviceLocked=${keyguard.isDeviceLocked}")
        } catch (_: Exception) { observation.set("failed") }
        finally { done.countDown() }
    }
    return if (done.await(500, TimeUnit.MILLISECONDS)) observation.get() else "timeout"
}
