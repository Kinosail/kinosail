/* Demonstration only: no account, checkout, persistence, or payment requests. */
const el = (id) => document.getElementById(id);
const state = {set: catalog.sets[0], rank: 10, mode: 'both', family: 'living', billing: 'monthly'};
const requested = new URLSearchParams(location.search).get('set');
state.set = catalog.sets.find((set) => set.slug === requested) || catalog.sets[0];
const art = (family, suffix = '') => `art/${state.set.slug}/${family}-${String(state.rank).padStart(2, '0')}${suffix}.svg`;
catalog.sets.forEach((set) => el('collection-select').add(new Option(set.name, set.slug)));
catalog.ranks.forEach((rank, i) => el('rank-select').add(new Option(`${i + 1} · ${rank}`, String(i + 1))));
el('collection-select').value = state.set.slug;
el('rank-select').value = String(state.rank);
const families = () => state.mode === 'both' ? ['living', 'patron'] : state.mode === 'patron' ? ['patron'] : state.mode === 'new' ? [] : ['living'];
function renderNav() {
  const nav = el('rank-nav');
  nav.replaceChildren();
  nav.removeAttribute('title');
  if (!el('show-badge').checked || state.mode === 'new') {
    nav.textContent = state.mode === 'new' ? 'Support Kinosail' : 'Your support';
    nav.setAttribute('aria-label', nav.textContent);
    return;
  }
  const labels = [];
  families().sort((a, b) => a === state.family ? -1 : b === state.family ? 1 : 0).forEach((family) => {
    const image = new Image();
    image.src = art(family, '-small');
    image.alt = '';
    image.width = image.height = 30;
    nav.append(image);
    labels.push(`${catalog.ranks[state.rank - 1]} ${family === 'living' ? 'Living Standard' : 'Patron Order'}`);
  });
  nav.append(document.createTextNode(catalog.ranks[state.rank - 1]));
  nav.title = labels.join(' · ');
  nav.setAttribute('aria-label', `${labels.join(' and ')}. Open your honors.`);
}
function renderHero() {
  const fresh = state.mode === 'new';
  const archived = state.mode === 'archived';
  const familyName = state.family === 'living' ? 'Living Standard' : 'Patron Order';
  el('hero-badge').src = art(state.family);
  el('hero-badge').alt = `${catalog.ranks[state.rank-1]} ${familyName}, ${state.set.name}`;
  el('hero-family').textContent = fresh ? 'A mark of our gratitude' : familyName;
  el('honor-title').textContent = fresh ? 'You’re welcome here.' : catalog.ranks[state.rank - 1];
  el('gratitude').textContent = fresh ? 'Help Kinosail grow.' : 'Thank you, Alex.';
  el('thank-you').textContent = fresh ? 'If Kinosail has found a place in your home, you can help shape its future. Every contribution is appreciated.' : archived ? 'Thank you for the time you supported Kinosail. Your contribution is part of its story, and your earned recognition stays with you.' : 'Your support gives Kinosail room to grow. We’re grateful you’ve chosen to be part of its story.';
  el('support-record').textContent = fresh ? `A preview of ${catalog.ranks[state.rank - 1]}, one of our supporter honors.` : state.family === 'patron' ? 'Permanent recognition · Supported September 2026' : archived ? 'Supported March–September 2026 · 6 months of support' : 'Supporting since March 2026 · 6 months together';
  el('hero-badge').style.opacity = fresh ? '.85' : '1';
  el('certificate-download').href = art(state.family, '-certificate');
  el('certificate-preview').src = art(state.family, '-certificate');
  el('certificate-open').href = art(state.family, '-certificate');
  el('certificate-download').hidden = fresh;
  el('share-certificate').hidden = fresh;
  el('replay').hidden = fresh;
  el('earned').hidden = fresh;
  document.querySelector('.certificate-section').hidden = fresh;
  el('share-status').textContent = '';
}
function renderOwned() {
  const root = el('owned-badges');
  root.replaceChildren();
  families().forEach((family) => {
    const item = document.createElement('article');
    item.className = 'owned';
    const image = new Image();
    image.src = art(family);
    image.alt = '';
    image.width = image.height = 112;
    const text = document.createElement('div');
    const label = document.createElement('p');
    label.className = 'eyebrow';
    label.textContent = family === 'living' ? 'Living Standard' : 'Patron Order';
    const title = document.createElement('h3');
    title.textContent = catalog.ranks[state.rank - 1];
    const record = document.createElement('p');
    record.textContent = family === 'patron' ? 'Permanent · Yours to keep' : state.mode === 'archived' ? 'Past support · Always appreciated' : 'Active · 6 months of support';
    const button = document.createElement('button');
    button.textContent = family === state.family ? 'On display' : 'Display this honor';
    button.setAttribute('aria-pressed', String(family === state.family));
    button.addEventListener('click', () => {state.family = family; renderNav(); renderHero(); renderOwned();});
    text.append(label, title, record, button);
    item.append(image, text);
    root.append(item);
  });
}
function render() {renderNav(); renderHero(); renderOwned();}
el('collection-select').addEventListener('change', (event) => {
  const set = catalog.sets.find((item) => item.slug === event.target.value);
  if (set) {state.set = set; render();}
});
el('rank-select').addEventListener('change', (event) => {
  const rank = Number(event.target.value);
  if (Number.isInteger(rank) && rank >= 1 && rank <= 10) {state.rank = rank; render();}
});
el('state-select').addEventListener('change', (event) => {
  if (!['both', 'living', 'patron', 'archived', 'new'].includes(event.target.value)) return;
  state.mode = event.target.value;
  state.family = state.mode === 'patron' ? 'patron' : 'living';
  if (state.mode === 'new') {state.rank = 1; el('rank-select').value = '1';}
  render();
});
el('show-badge').addEventListener('change', renderNav);
el('replay').addEventListener('click', () => {
  const image = el('hero-badge');
  image.classList.remove('reveal');
  requestAnimationFrame(() => requestAnimationFrame(() => image.classList.add('reveal')));
});
el('share-certificate').addEventListener('click', async () => {
  const status = el('share-status');
  try {
    const response = await fetch(art(state.family, '-certificate'));
    if (!response.ok) throw new Error('Certificate unavailable');
    const file = new File([await response.blob()], 'kinosail-sample-certificate.svg', {type: 'image/svg+xml'});
    if (navigator.canShare?.({files: [file]})) {
      await navigator.share({files: [file], title: 'Kinosail sample supporter certificate'});
      status.textContent = 'Certificate shared.';
    } else {
      el('certificate-download').click();
      status.textContent = 'Certificate downloaded. You can share the saved SVG file.';
    }
  } catch (error) {
    status.textContent = error.name === 'AbortError' ? 'Sharing canceled. Your certificate is still here.' : 'Sharing is unavailable here. Use “Keep your certificate” to save the SVG.';
  }
});
function pricing() {
  const prices = catalog[state.billing];
  const suffix = state.billing === 'monthly' ? '/month' : state.billing === 'annual' ? '/year' : ' once';
  const note = state.billing === 'monthly' ? 'Regular support helps us plan ahead. Cancel whenever you need to.' : state.billing === 'annual' ? 'One charge each year. Friend starts at $12/year, equivalent to $1/month.' : 'A single contribution. Your Patron Order is permanent.';
  el('billing-note').textContent = note;
  document.querySelectorAll('[data-billing]').forEach((button) => button.setAttribute('aria-pressed', String(button.dataset.billing === state.billing)));
  for (const target of ['entry-prices', 'price-list']) {
    const root = el(target);
    root.replaceChildren();
    const indices = target === 'entry-prices' ? [0, 1, 2] : catalog.ranks.map((_, i) => i);
    indices.forEach((i) => {
      const button = document.createElement('button');
      const title = document.createElement('span');
      title.textContent = catalog.ranks[i];
      const amount = document.createElement('strong');
      amount.textContent = `$${prices[i]}${suffix}`;
      button.append(title, amount);
      if (target === 'entry-prices') {
        button.className = 'price-choice';
        const billing = document.createElement('small');
        billing.textContent = state.billing === 'annual' ? `$${prices[i]} billed annually` : state.billing === 'monthly' ? 'Billed monthly' : 'Permanent recognition';
        button.append(billing);
      }
      button.addEventListener('click', () => {el('price-status').textContent = `${catalog.ranks[i]} selected at $${prices[i]}${suffix}. This is a design preview; no checkout or charge occurs.`;});
      root.append(button);
    });
  }
}
document.querySelectorAll('[data-billing]').forEach((button) => button.addEventListener('click', () => {state.billing = button.dataset.billing; pricing();}));
render();
pricing();
