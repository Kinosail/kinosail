// Use the acknowledged public settings adapter for both explicit capabilities.
export async function withRecoveryControls(page, save, use) {
  await page.goto('/settings');
  const initial = await page.getByLabel('Playback subtitle choices').inputValue();
  if (!['on', 'off'].includes(initial)) throw Error('invalid recovery controls owner');
  let failed = false;
  try {
    for (const [choice, mode] of [['on', 'custom'], ['off', 'native']]) {
      await page.goto('/settings');
      await save(page, choice);
      await use(mode);
    }
  } catch (error) { failed = true; throw error; } finally {
    try { await page.goto('/settings'); await save(page, initial); }
    catch (error) { if (!failed) throw error; }
  }
}
