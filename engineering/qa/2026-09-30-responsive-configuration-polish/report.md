# Responsive Player configuration checkpoint — September 30, 2026

The populated audit confirmed two responsive defects. Search text briefly faded through unreadable colors when the landscape field collapsed. TMDB connection instructions inherited horizontal wizard styling, hiding later steps on phones.

The landscape Search input now changes text color immediately while retaining background and border transitions. TMDB instructions use a normal ordered list. The Player stylesheet version is `electric-39` so browsers fetch the change.

The existing configuration accessibility regression failed before the Search fix. It reported 1.02:1 contrast at 720px, below its 4.5:1 requirement. Its assertion now retains the complete accessibility violation instead of only its ID. A new TMDB regression failed before the list fix: the second instruction extended to 749.64px in a 390px viewport. That regression now runs in the populated smoke suite.

See the [before and after gallery](gallery.html) and [repeatable run context](run-context.json). The gallery uses synthetic data. The before capture shows the full viewport; the after captures show the TMDB section.

| Check | Result |
| --- | --- |
| Configuration, navigation, and canvas checks | 18/18 passed across Chromium, Firefox, and WebKit. |
| TMDB regression in its final smoke-suite location | 3/3 passed across the same browsers. |
| Supported configuration widths | 1440, 1024, 900, 720, 390, and 320px. |
| Full Player Go suite | All packages passed; server package completed in 134.678s. |
| `make -C apps/player verify-changed` | Passed compilation, focused Go tests, UI lint, test discovery, source cap, and diff check. |
| `make max-loc` and `make tooling-check` | Passed. |
| Local `make -C apps/player test-instance-check` | Blocked before startup: shared Podman storage exhausted. No unrelated resources were removed. |

The broad baseline audit remains separate. Its initial refreshed run passed 3/13 checks. Other failures concern older navigation selectors and expectations, settings-search state, transcoder wording, and two geometry assertions. Those checks remain under review. This focused result does not certify the whole layout suite, physical devices, long playback, or a Nox deployment. No asynchronous production surface changed in this batch.

The preceding Android fix was merged through [PR #383](https://github.com/Kinosail/kinosail/pull/383). Fetched ancestry proved its reviewed head is included in remote main `6e63f1cfffe5240804a369383d0afae9847e5311`. Its first main CI attempt failed before a subtitle test assertion because Linux could not execute a temporary FFmpeg script: `text file busy`. Thirty local repetitions passed with unchanged code. One rerun of the same main revision passed all checks, Player/Subtitles container publication, and production-tag promotion. See [main run 36752448363](https://github.com/Kinosail/kinosail/actions/runs/36752448363).

Current main's catalog changes were reconciled before publication. The combined browser matrix passed 12/12 checks. The first combined Go run failed during temporary-directory cleanup after metadata assertions; three focused repetitions and the next full run passed. The final server run took 138.438s. The changed-app and tooling gates passed again. The cleanup failure remains recorded, without an unconfirmed source fix.

The primary checkout's unrelated dirty files were preserved. They prevent automatic local-main cleanup; this task continues in its leased worktree.
