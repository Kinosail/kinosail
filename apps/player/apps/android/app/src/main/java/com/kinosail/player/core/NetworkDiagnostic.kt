package com.kinosail.player.core

internal fun diagnosticOperation(rawPath: String): String {
    if (rawPath.length > 2048) return "other"
    val parts = rawPath.substringBefore('?').split('/').filter(String::isNotEmpty)
    if (parts.size >= 3 && parts[0] == "api" && parts[1] == "v1") {
        if (parts[2] == "items" && parts.size >= 5 && parts[4] in setOf(
                "playback", "playback-preferences", "progress", "list", "reader")) {
            return "items-${parts[4]}"
        }
        return if (parts[2] in setOf("items", "library", "me", "session", "quick-connect", "shows",
                "albums", "collections", "books", "cast", "remote-players", "downloads")) parts[2] else "api-other"
    }
    return if (parts.isNotEmpty() && parts[0] in setOf("hls", "media", "art")) parts[0] else "other"
}
