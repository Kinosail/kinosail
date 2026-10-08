"""Source contracts for resource-held public browser recipes; no runtime qualification."""
from pathlib import Path
import unittest
ROOT=Path(__file__).resolve().parents[2]
class BrowserCapabilities(unittest.TestCase):
 def test_required_player_command_runs_both_new_browser_controls_once(self):
  command=next(line for line in (ROOT/'.github/workflows/app.yml').read_text().splitlines() if 'run: node --test scripts/testing/player-setup-navigation.test.mjs' in line)
  for name in ['scripts/testing/playback-state-witness.test.mjs','scripts/testing/recovery-controls-owner.test.mjs']:
   self.assertEqual(command.split().count(name),1,'required browser callback control must execute once')
 def test_apple_cold_firsttap_precedes_held_media_admission(self):
  source=(ROOT/'apps/player/e2e/playback-startup-blocked.spec.ts').read_text()
  cold=source.index('if (browserName === "webkit") {',source.index('phase = "pending"'))
  admission=source.index('await expect.poll(() => hold.snapshot()',cold)
  self.assertLess(source.index('await page.locator(".player-center-control[data-player-toggle]").click()',cold),admission)
  self.assertTrue('appleNativePlayback' in source[cold:admission], 'required capability contract missing')
 def test_desktop_pending_and_native_owners_are_explicit(self):
  source=(ROOT/'apps/player/e2e/playback-startup-blocked.spec.ts').read_text()
  self.assertTrue('if (browserName !== "webkit") {' in source, 'required capability contract missing')
  self.assertTrue('await expect(page.locator("[data-player-status]")).toBeVisible()' in source, 'required capability contract missing')
  recovery=(ROOT/'apps/player/e2e/layout-audit-player-recovery.spec.ts').read_text()
  self.assertTrue('withRecoveryControls(page, saveSubtitleChoices' in recovery, 'required capability contract missing')
  self.assertTrue('if (mode === "custom")' in recovery, 'required capability contract missing')
  self.assertTrue('await expectTheaterEditingGuard(page)' in recovery, 'required capability contract missing')
  self.assertTrue('await fullscreen.click()' in recovery, 'required capability contract missing')
  self.assertTrue('await expect(fullscreen).toBeDisabled()' in recovery, 'required capability contract missing')
