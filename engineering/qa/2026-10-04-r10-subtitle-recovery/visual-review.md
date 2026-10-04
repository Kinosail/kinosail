# R10 responsive state inspection

Actual Go-rendered screenshots at tested revision `6af39f2f` were inspected in
one batch, using unfinished-body cases at 390, 1440 and 1920 pixels. Both
header/body paths retain 24 original screenshots. No product change was needed
after inspection.

| State | Observed behavior |
|---|---|
| Pending | Real video plays, English stays selected, Loading subtitles is present, Retry is absent. |
| Failed | Clear bounded failure text wraps within the caption row; quiet Retry is visible and usable. Video continues and English remains selected. |
| Loaded | Recovered captions visibly appear over later video frames. Pending/failure text and Retry disappear; Electric focus returns to the selector. |
| Empty / Off | Off is selected, captions disappear from the video and status/Retry remain hidden. |

The caption row expands only for real loading/failure text and its recovery
action. The video stage keeps its geometry; no skeleton or placeholder blocks
playback or remains after completion. Phone controls/text remain within the
viewport, and the desktop/TV layouts retain the existing bounded player width.
Failed-state screenshots capture the current scrolled viewport; the other
states are full-page captures. TV here means a 1920-pixel browser viewport,
not a physical TV or native tvOS proof.
