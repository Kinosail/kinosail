(() => {
  const root = document.documentElement;
  const theme = document.querySelector('[data-theme-toggle]');
  const menu = document.querySelector('[data-menu-toggle]');
  const sidebar = document.querySelector('[data-sidebar]');
  const form = document.querySelector('[data-search-form]');
  const input = document.querySelector('[data-search-input]');
  const results = document.querySelector('[data-search-results]');
  let pages, pending, revision = 0;
  const currentTheme = () => root.dataset.theme || (matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark');
  const labelTheme = () => theme?.setAttribute('aria-label', `Use ${currentTheme() === 'dark' ? 'light' : 'dark'} theme`);
  const mobile = matchMedia('(max-width: 850px)');
  const main = document.querySelector('main');
  function setMenu(open) {
    sidebar?.classList.toggle('is-open', open);
    menu?.setAttribute('aria-expanded', String(open));
    if (main) main.inert = open && mobile.matches;
    if (open) input?.focus();
  }
  const closeMenu = () => setMenu(false);
  mobile.addEventListener('change', closeMenu);
  theme?.addEventListener('click', () => {
    root.dataset.theme = currentTheme() === 'dark' ? 'light' : 'dark';
    try { localStorage.setItem('kinosail-docs-theme', root.dataset.theme); } catch { /* Storage is optional. */ }
    labelTheme();
  });
  labelTheme();
  menu?.addEventListener('click', () => setMenu(!sidebar.classList.contains('is-open')));
  function message(text) {
    results.replaceChildren(Object.assign(document.createElement('small'), { textContent: text }));
    results.hidden = false;
  }
  async function loadIndex() {
    const response = await fetch(form.dataset.indexUrl);
    if (!response.ok) throw new Error('Search unavailable');
    const text = await response.text();
    if (text.length > 3000000) throw new Error('Search too large');
    const entries = JSON.parse(text);
    if (!Array.isArray(entries) || entries.length > 1000) throw new Error('Invalid index');
    const prefix = new URL(form.dataset.indexUrl, location.href).pathname.replace(/search.json$/, '');
    if (!entries.every(page => page && ['title', 'description', 'url', 'content', 'product'].every(key => typeof page[key] === 'string' && page[key].length < 100000) &&
      page.url.startsWith(prefix) && !page.url.startsWith('//') && !page.url.includes('\\') &&
      new URL(page.url, location.origin).origin === location.origin && new URL(page.url, location.origin).pathname.startsWith(prefix))) throw new Error('Invalid entry');
    return entries;
  }
  async function search() {
    const request = ++revision;
    const query = input.value.trim().toLocaleLowerCase().slice(0, 200);
    if (query.length < 2) { results.hidden = true; return; }
    message('Searching…');
    try {
      if (!pages) {
        pending ||= loadIndex().finally(() => { pending = null; });
        pages = await pending;
      }
      if (request !== revision) return;
      const terms = query.split(/\s+/).slice(0, 20);
      const matches = pages.map(page => ({ page, score: terms.reduce((score, term) => score + (page.title.toLocaleLowerCase().includes(term) ? 3 : 0), 0) }))
        .filter(({ page }) => terms.every(term => `${page.product} ${page.title} ${page.description} ${page.content}`.toLocaleLowerCase().includes(term)))
        .sort((a, b) => b.score - a.score).slice(0, 10);
      if (!matches.length) { message('No matching pages. Try an app name or a shorter search.'); return; }
      results.replaceChildren(...matches.map(({ page }) => {
        const link = document.createElement('a');
        link.href = page.url;
        const title = Object.assign(document.createElement('strong'), { textContent: page.title });
        const detail = Object.assign(document.createElement('small'), { textContent: `${page.product} · ${page.description}` });
        link.append(title, detail);
        return link;
      }));
      results.hidden = false;
    } catch {
      if (request === revision) message('Search could not load. Try again, or browse the navigation below.');
    }
  }
  input?.addEventListener('input', search);
  input?.addEventListener('keydown', event => {
    if (event.key === 'ArrowDown') { event.preventDefault(); results.querySelector('a')?.focus(); }
  });
  results?.addEventListener('keydown', event => {
    if (!['ArrowDown', 'ArrowUp'].includes(event.key)) return;
    event.preventDefault();
    const links = [...results.querySelectorAll('a')];
    const index = links.indexOf(document.activeElement) + (event.key === 'ArrowDown' ? 1 : -1);
    (index < 0 ? input : links[Math.min(index, links.length - 1)])?.focus();
  });
  form?.addEventListener('submit', event => { event.preventDefault(); if (!results.hidden) results.querySelector('a')?.click(); });
  document.addEventListener('click', event => { if (form && !form.contains(event.target)) { revision++; results.hidden = true; } });
  document.addEventListener('keydown', event => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      if (mobile.matches) setMenu(true);
      input.focus();
    }
    if (event.key === 'Escape') {
      revision++;
      if (!results.hidden) { results.hidden = true; input.focus(); }
      else if (sidebar?.classList.contains('is-open')) { closeMenu(); menu.focus(); }
    }
  });
  document.querySelectorAll('.prose pre').forEach(pre => {
    const code = pre.querySelector('code');
    if (!code || !navigator.clipboard) return;
    const button = Object.assign(document.createElement('button'), { type: 'button', className: 'copy-button', textContent: 'Copy' });
    button.setAttribute('aria-label', 'Copy code');
    button.setAttribute('aria-live', 'polite');
    button.addEventListener('click', async () => {
      try { await navigator.clipboard.writeText(code.textContent); button.textContent = 'Copied'; }
      catch { button.textContent = 'Select and copy manually'; }
      setTimeout(() => { button.textContent = 'Copy'; }, 2500);
    });
    pre.append(button);
  });
  const toc = document.querySelector('[data-toc]');
  const headings = document.querySelectorAll('.prose h2[id]');
  if (toc && headings.length > 1) {
    headings.forEach(heading => {
      const link = Object.assign(document.createElement('a'), { textContent: heading.textContent, href: `#${heading.id}` });
      toc.querySelector('nav').append(link);
    });
    toc.hidden = false;
  }
})();
