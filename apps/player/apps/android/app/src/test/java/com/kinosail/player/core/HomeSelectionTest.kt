package com.kinosail.player.core

import org.junit.Assert.*
import org.junit.Test

class HomeSelectionTest {
    private fun item(id: String, kind: String = "video", genres: String = "") =
        CatalogItem(id, kind, id, "", "", "", genres = genres)

    @Test fun watchingExcludesAudioWatchedAndDismissedAndCapsTheContinuation() {
        val history = (1..20).map { item("film-$it").copy(progress = WatchProgress(seconds = 42.0)) }
        val state = HomeState(listOf(item("song", "music").copy(progress = WatchProgress(seconds = 10.0)), item("done").copy(progress = WatchProgress(watched = true)),
            item("hidden").copy(progress = WatchProgress(seconds = 10.0, dismissed = true))) + history)
        assertEquals("film-1", state.featured?.id)
        assertEquals((2..16).map { "film-$it" }, state.watchShelf.map { it.id })
        assertEquals((1..15).map { "film-$it" }, state.tvWatchingRail.map { it.id })
        assertEquals("song", state.copy(listening = true).featured?.id)
    }

    @Test fun recentShelvesAndGenresHaveTheSameMeaningAsAppleHome() {
        val movie = item("movie", genres = "Drama · Science fiction · Drama")
        val episode = item("episode", genres = "Drama").copy(showId = "0123456789abcdef")
        val state = HomeState(recent = listOf(movie, episode, item("show", "show"), item("song", "music"),
            item("watched").copy(progress = WatchProgress(watched = true))))
        assertEquals(listOf("movie", "watched"), state.movies.map { it.id })
        assertEquals(listOf("episode", "show"), state.shows.map { it.id })
        assertEquals(listOf("movie"), state.unwatchedMovies.map { it.id })
        assertEquals(listOf("Drama", "Science fiction"), state.movieGenres.map { it.first })
        assertEquals(listOf(movie), state.movieGenres.first().second)
        assertEquals(listOf("song"), state.copy(listening = true).recentShelf.map { it.id })
        assertNull(HomeState(recent = listOf(item("photo", "photo"))).featured)
    }
}
