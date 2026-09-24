package com.kinosail.player.core

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class MediaTimelineTest {
    @Test fun mapsServerOmittedRangesAndResume() {
        val timeline = MediaTimeline(120.0, 100.0, listOf(OmittedRange(20.0, 30.0), OmittedRange(70.0, 80.0)))
        assertEquals(15.0, timeline.sourceTime(15.0), 0.0)
        assertEquals(30.0, timeline.sourceTime(20.0), 0.0)
        assertEquals(80.0, timeline.sourceTime(60.0), 0.0)
        assertEquals(60.0, timeline.presentationTime(75.0), 0.0)
        assertEquals(100.0, timeline.presentationTime(120.0), 0.0)
    }

    @Test fun rejectsMalformedAndConflictingTimelines() {
        listOf("{}", """{"sourceDuration":120,"duration":100,"omitted":[{"start":20,"end":10}]}""",
            """{"sourceDuration":120,"duration":99,"omitted":[{"start":20,"end":40}]}""",
            """{"sourceDuration":120,"duration":100,"omitted":[{"start":20,"end":30},{"start":25,"end":35}]}""",
            """{"sourceDuration":120,"duration":100,"omitted":[{"start":20,"end":30,"extra":1}]}""",
            """{"sourceDuration":"120","duration":100}""").forEach {
            assertThrows(it, Exception::class.java) { MediaTimeline.parse(StrictJson.parse(it)) }
        }
    }
}
