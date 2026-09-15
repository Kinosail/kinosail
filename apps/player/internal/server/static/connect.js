const appLink = document.querySelector('[data-connect-app]');
if (appLink && /^[0-9]{6}$/.test(appLink.dataset.code ?? '')) {
  const query = new URLSearchParams({
    server: window.location.origin,
    code: appLink.dataset.code,
  });
  appLink.href = `kinosail://approve?${query}`;
  appLink.hidden = false;
}
