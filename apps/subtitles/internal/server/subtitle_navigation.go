package server

const subtitleAppHeader = `<header class="app-header">
<a class="brand-lockup" href="/" data-subtitle-nav><img class="brand-icon" src="/static/icon.svg?v=11" width="36" height="36" alt=""><span class="subtitle-brand-name">{{.ServerName}}<small>Subtitle library</small></span></a>
<nav aria-label="Main navigation">
<a href="/?view=summary" data-subtitle-nav {{if eq .View "summary"}}class="active" aria-current="page"{{end}}>Overview</a>
<a href="/?view=wanted" data-subtitle-nav {{if eq .View "wanted"}}class="active" aria-current="page"{{end}}>Wanted</a>
<a href="/?view=library" data-subtitle-nav {{if eq .View "library"}}class="active" aria-current="page"{{end}}>Library</a>
</nav>
<div class="subtitle-header-settings"><a class="header-link" href="/settings" {{if eq .View "settings"}}aria-current="page"{{end}}>Settings</a></div>
</header>`
