package com.kinosail.player.core

import com.fasterxml.jackson.core.JsonFactory
import com.fasterxml.jackson.core.StreamReadConstraints
import com.fasterxml.jackson.core.StreamReadFeature
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement

object StrictJson {
    private val factory = JsonFactory.builder()
        .streamReadConstraints(StreamReadConstraints.builder()
            .maxNestingDepth(64).maxNameLength(256).maxStringLength(65_536).maxNumberLength(32).build())
        .enable(StreamReadFeature.STRICT_DUPLICATE_DETECTION)
        .build()

    fun parse(text: String): JsonElement {
        factory.createParser(text).use { parser -> while (parser.nextToken() != null) Unit }
        return Json.parseToJsonElement(text)
    }
}
