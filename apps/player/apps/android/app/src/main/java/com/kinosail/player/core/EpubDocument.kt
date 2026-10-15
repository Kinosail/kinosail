package com.kinosail.player.core

import android.content.Context
import android.graphics.Color
import android.webkit.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.viewinterop.AndroidView

@Composable
internal fun EpubDocument(book: EpubBook, saved: SavedSession, viewer: Viewer, position: ReaderPosition,
    fontSize: Int, light: Boolean, modifier: Modifier, result: (Boolean) -> Unit, offset: (Double) -> Unit,
    navigate: (Int) -> Unit) {
    val currentOffset by rememberUpdatedState(offset)
    val currentResult by rememberUpdatedState(result)
    val currentNavigate by rememberUpdatedState(navigate)
    val initialOffset = remember { position.offset }
    AndroidView(modifier = modifier.clipToBounds(), factory = { context ->
        val resources = EpubResources(book.id, saved, viewer, fontSize, light)
        val chapterView = ChapterWebView(context)
        chapterView.settings.setAllowContentAccess(false)
        with(chapterView) {
            contentDescription = "Book chapter"
            setBackgroundColor(if (light) Color.rgb(244, 248, 239) else Color.rgb(11, 13, 11))
            settings.textZoom = 100
            settings.javaScriptEnabled = false
            settings.domStorageEnabled = false
            settings.allowFileAccess = false
            settings.blockNetworkLoads = true
            settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
            settings.cacheMode = WebSettings.LOAD_NO_CACHE
            settings.setSupportMultipleWindows(false)
            settings.setSupportZoom(true)
            settings.builtInZoomControls = true
            settings.displayZoomControls = false
            setDownloadListener { _, _, _, _, _ -> }
            CookieManager.getInstance().setAcceptThirdPartyCookies(this, false)
            webViewClient = object : WebViewClient() {
                override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse {
                    if (request.method != "GET") return EpubResources.blocked()
                    val response = resources.load(request.url.toString())
                    if (request.isForMainFrame && response.statusCode != 200) {
                        post { if (active) { failed = true; loaded = false; currentResult(false) } }
                    }
                    return response
                }
                override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                    val path = EpubResources.path(request.url.toString(), book.id)
                    val target = book.chapters.indexOfFirst { it.path == path }
                    if (request.isForMainFrame && request.hasGesture() && target >= 0 && request.method == "GET") {
                        if (target + 1 == position.page) return false
                        currentNavigate(target + 1)
                    }
                    return true
                }
                override fun onPageFinished(view: WebView, url: String) {
                    if (!active || url != EpubResources.ORIGIN + book.chapters[position.page - 1].path) return
                    postVisualStateCallback(0, object : WebView.VisualStateCallback() {
                        override fun onComplete(requestId: Long) {
                            if (!active || failed) return
                            restore(initialOffset)
                            loaded = true
                            currentResult(true)
                        }
                    })
                }
                override fun onReceivedHttpError(view: WebView, request: WebResourceRequest, response: WebResourceResponse) {
                    if (request.isForMainFrame && active) { failed = true; currentResult(false) }
                }
                override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
                    if (request.isForMainFrame && active) { failed = true; currentResult(false) }
                }
                override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean {
                    android.util.Log.e("KinosailReader", "Renderer stopped operation=reader-render kind=terminated outcome=retryable")
                    (parent as? android.view.ViewGroup)?.removeView(chapterView)
                    release()
                    currentResult(false)
                    return true
                }
            }
            changed = { currentOffset(it) }
            loadUrl(EpubResources.ORIGIN + book.chapters[position.page - 1].path)
        }
        chapterView
    }, onRelease = { it.release() })
}

private class ChapterWebView(context: Context) : WebView(context) {
    var active = true
    var loaded = false
    var failed = false
    var changed: (Double) -> Unit = {}
    fun release() { if (active) { active = false; stopLoading(); destroy() } }
    fun restore(offset: Double) { scrollTo(0, ((computeVerticalScrollRange() - height).coerceAtLeast(0) * offset).toInt()) }
    override fun onScrollChanged(left: Int, top: Int, oldLeft: Int, oldTop: Int) {
        super.onScrollChanged(left, top, oldLeft, oldTop)
        val distance = (computeVerticalScrollRange() - height).coerceAtLeast(0)
        if (active && loaded && distance > 0) changed((top.toDouble() / distance).coerceIn(0.0, 1.0))
    }
}
