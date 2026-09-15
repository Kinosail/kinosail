/* Local design explorer. Only allowlisted, generated artwork paths are used. */
const el = (id) => document.getElementById(id);
const artPath = (set, family, rank, suffix = '') => `art/${set.slug}/${family}-${String(rank).padStart(2, '0')}${suffix}.svg`;
const dialog = el('art-dialog');
function openBadge(set, family, rank) {
  const label = `${catalog.ranks[rank - 1]} · ${set.name}`;
  el('dialog-title').textContent = label;
  el('dialog-family').textContent = family === 'living' ? 'Living Standard · Recurring' : 'Patron Order · One-time';
  el('dialog-art').src = artPath(set, family, rank);
  el('dialog-art').alt = label;
  el('dialog-download').href = artPath(set, family, rank);
  el('dialog-certificate').href = artPath(set, family, rank, '-certificate');
  dialog.showModal();
}
dialog.querySelector('.close').addEventListener('click', () => dialog.close());
function renderCollection(index) {
  const set = catalog.sets[index];
  el('collection-number').textContent = `Collection ${String(index + 1).padStart(2, '0')} / 10`;
  el('collection-name').textContent = set.name;
  el('collection-note').textContent = set.note;
  el('sheet-download').href = `art/${set.slug}/collection.svg`;
  el('page-preview').href = `supporter.html?set=${set.slug}`;
  document.querySelectorAll('.direction').forEach((button, i) => button.setAttribute('aria-pressed', String(i === index)));
  for (const family of ['living', 'patron']) {
    const grid = el(`${family}-grid`);
    grid.replaceChildren();
    catalog.ranks.forEach((name, i) => {
      const rank = i + 1;
      const card = document.createElement('article');
      card.className = 'badge-card';
      const button = document.createElement('button');
      button.className = 'badge-open';
      button.setAttribute('aria-label', `Enlarge ${name} ${family === 'living' ? 'Living Standard' : 'Patron Order'}`);
      const image = new Image();
      image.src = artPath(set, family, rank);
      image.alt = '';
      image.width = image.height = 360;
      button.append(image);
      button.addEventListener('click', () => openBadge(set, family, rank));
      const title = document.createElement('h4');
      title.textContent = `${String(rank).padStart(2, '0')} · ${name}`;
      const price = document.createElement('p');
      price.textContent = family === 'living' ? `$${catalog.monthly[i]}/month · $${catalog.annual[i]}/year` : `$${catalog.once[i]} one-time`;
      const small = document.createElement('div');
      small.className = 'micro-preview';
      const mini = new Image();
      mini.src = artPath(set, family, rank, '-small');
      mini.width = mini.height = 32;
      mini.alt = `${name} compact badge`;
      small.append(mini, document.createTextNode(name));
      card.append(button, title, price, small);
      grid.append(card);
    });
  }
}
catalog.sets.forEach((set, i) => {
  const button = document.createElement('button');
  button.className = 'direction';
  button.setAttribute('aria-pressed', String(i === 0));
  const image = new Image();
  image.src = artPath(set, 'patron', 10);
  image.alt = '';
  image.width = image.height = 360;
  const title = document.createElement('strong');
  title.textContent = `${String(i + 1).padStart(2, '0')} ${set.name}`;
  const caption = document.createElement('span');
  caption.textContent = i === 0 ? 'Recommended starting point' : 'Explore both families →';
  button.append(image, title, caption);
  button.addEventListener('click', () => {
    renderCollection(i);
    el('collection').scrollIntoView({behavior: 'instant', block: 'start'});
  });
  el('directions').append(button);
});
renderCollection(0);
