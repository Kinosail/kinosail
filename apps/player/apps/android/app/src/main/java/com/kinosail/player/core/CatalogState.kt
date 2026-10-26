package com.kinosail.player.core

data class CatalogLetter(val label: String, val offset: Int, val count: Int)

data class CatalogState(
    val items: List<CatalogItem> = emptyList(),
    val total: Int = 0,
    val loading: Boolean = false,
    val notice: String? = null,
    val failedOffset: Int? = null,
    val connectionExpired: Boolean = false,
    val resultQuery: String? = null,
    val selected: CatalogItem? = null,
    val view: String = "all",
    val listed: Boolean? = null,
    val listBusy: Boolean = false,
    val detailNotice: String? = null,
    val sort: String = "title",
    val letters: List<CatalogLetter> = emptyList(),
)

internal fun catalogEmptyMessage(view: String, searched: Boolean): String =
    if (searched) "No results. Try another search."
    else if (view == "list") "Save a title to keep it in My List." else "Nothing in your library yet."
