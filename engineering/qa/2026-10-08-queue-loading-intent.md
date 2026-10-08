# Queue loading intent regression

A source review of PR503 `fdc08dd7` suspected that seek or Stop during a queued
track load can be overwritten by saved progress when real metadata arrives.
The complete queue source blob `b21d38111a94b249afb083b1656dcd0e4f8af7b9` is identical
in that candidate, PR523 `e11212eb`, and published main `63f38a09`.
This is a source suspicion before the first hosted browser result.

This first checkpoint changes tests and their required selection only.
The existing fictional two-track album, all prior tests and production stay intact.
The new journeys save five seconds through the public progress API, delay the
next track's real media request, and queue seek or Stop through the public API.
They require real command consumption and actual browser position before metadata.
After releasing the unchanged media request, they observe the real loadedmetadata
event and require the latest intent to survive. There is no synthetic media or
metadata event. The request delay is explicit fault injection in a real Server.

The first attempt and every retry must remain attributable to their exact source.
Prerequisite failures do not prove the position-overwrite suspicion. Production
changes require a confirmed failure and independent review. Preserve native
intent, preparation, all existing failure controls, scanner/Q12 holds and PR517
ownership. This work performs no native/UI/local build or manual deployment.
