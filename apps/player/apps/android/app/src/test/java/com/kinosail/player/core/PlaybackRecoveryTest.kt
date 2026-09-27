package com.kinosail.player.core

import android.net.Uri
import androidx.media3.common.PlaybackException
import androidx.media3.datasource.DataSpec
import androidx.media3.datasource.HttpDataSource
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class PlaybackRecoveryTest {
    @Test fun restartsRetryOnlyTemporaryMediaFailures() {
        assertTrue(PlaybackModel.isTransientMediaFailure(PlaybackException("timeout", null,
            PlaybackException.ERROR_CODE_IO_NETWORK_CONNECTION_TIMEOUT)))
        assertTrue(PlaybackModel.isTransientMediaFailure(PlaybackException("disconnect", null,
            PlaybackException.ERROR_CODE_IO_NETWORK_CONNECTION_FAILED)))
        for (status in listOf(503, 502, 504)) {
            val response = HttpDataSource.InvalidResponseCodeException(status, "unavailable", null,
                emptyMap(), DataSpec(Uri.parse("https://example.com/media/item")), byteArrayOf())
            assertTrue(PlaybackModel.isTransientMediaFailure(PlaybackException("server", response,
                PlaybackException.ERROR_CODE_IO_BAD_HTTP_STATUS)))
        }
        val denied = HttpDataSource.InvalidResponseCodeException(401, "denied", null,
            emptyMap(), DataSpec(Uri.parse("https://example.com/media/item")), byteArrayOf())
        assertFalse(PlaybackModel.isTransientMediaFailure(PlaybackException("auth", denied,
            PlaybackException.ERROR_CODE_IO_BAD_HTTP_STATUS)))
        assertFalse(PlaybackModel.isTransientMediaFailure(PlaybackException("format", null,
            PlaybackException.ERROR_CODE_PARSING_CONTAINER_UNSUPPORTED)))
    }
}
