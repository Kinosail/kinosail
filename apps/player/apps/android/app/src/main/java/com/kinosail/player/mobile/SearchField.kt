package com.kinosail.player.mobile

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import com.kinosail.player.core.CatalogModel
import com.kinosail.player.core.interfaceText
import com.kinosail.player.design.KinoColor
import java.text.NumberFormat
import kotlinx.coroutines.delay

@Composable
internal fun SearchField(catalog: CatalogModel, hideKeyboard: () -> Unit) {
    LaunchedEffect(catalog.searchInput) {
        if (catalog.searchInput.trim() == catalog.activeSearch) return@LaunchedEffect
        delay(250)
        catalog.search()
    }
    OutlinedTextField(catalog.searchInput, onValueChange = { if (it.length <= 512) catalog.searchInput = it },
        placeholder = { Text(interfaceText("Search your library")) }, singleLine = true,
        leadingIcon = { Icon(Icons.Filled.Search, contentDescription = null) },
        trailingIcon = {
            if (catalog.searchInput.isNotEmpty()) IconButton(onClick = { catalog.clearSearch(); hideKeyboard() }) {
                Icon(Icons.Filled.Close, contentDescription = interfaceText("Clear search"))
            }
        },
        shape = RoundedCornerShape(28.dp),
        colors = OutlinedTextFieldDefaults.colors(unfocusedContainerColor = KinoColor.surface,
            focusedContainerColor = KinoColor.surface, unfocusedBorderColor = KinoColor.surface),
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
        keyboardActions = KeyboardActions(onSearch = { catalog.search(); hideKeyboard() }),
        modifier = Modifier.fillMaxWidth())
}

@Composable
internal fun TitleCount(total: Int) {
    Text("${NumberFormat.getIntegerInstance().format(total)} ${interfaceText(if (total == 1) "title" else "titles")}",
        style = MaterialTheme.typography.bodyMedium, color = KinoColor.muted,
        modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
}
