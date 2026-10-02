package com.kinosail.player.core

import android.content.Context
import java.security.MessageDigest

internal class PersonalTabs(context: Context) {
    private val preferences = context.getSharedPreferences("kinosail_tabs", Context.MODE_PRIVATE)
    fun load(viewer: Viewer): List<String> {
        val preferenceKey = key(viewer)
        val saved = try { preferences.getString(preferenceKey, null) }
            catch (_: ClassCastException) { null } ?: return defaults
        if (saved.length > 128) return defaults
        return saved.split(',').takeIf(::valid) ?: defaults
    }
    fun save(viewer: Viewer, tabs: List<String>) {
        require(valid(tabs)) { "Choose one to four different destinations." }
        check(preferences.edit().putString(key(viewer), tabs.joinToString(",")).commit()) {
            "Could not save your tabs. Try again."
        }
    }
    private fun key(viewer: Viewer): String {
        viewer.validated()
        return MessageDigest.getInstance("SHA-256").digest("${viewer.serverId}\n${viewer.id}".toByteArray())
            .joinToString("") { "%02x".format(it) }
    }
    companion object {
        val defaults = listOf("home", "shows", "movies", "search")
        val destinations = listOf("home" to "Home", "shows" to "TV Shows", "movies" to "Movies",
            "search" to "Search", "listen" to "Listen", "music" to "Music", "audiobooks" to "Audiobooks",
            "books" to "Books", "photos" to "Photos", "list" to "My List", "all" to "Library")
        private fun valid(tabs: List<String>) = tabs.size in 1..4 && tabs.distinct().size == tabs.size &&
            tabs.all { id -> destinations.any { it.first == id } }
    }
}
