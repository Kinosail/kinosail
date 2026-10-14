# Keep settings categories beside their controls

The populated configuration audit exposed misplaced desktop category navigation.
When a restart notice is present, the menu appears below the entire content column.
The content uses a fixed grid row, while the notice and navigation use automatic placement.

The regression extends the existing real single sign-on save journey.
It compares Settings before and after that save at 320, 390, 900, 901, 1024, 1440, and 1920 pixels.
Desktop categories must align beside content, below the level controls and restart notice.
Mobile categories must remain above the content.
Each observation retains numeric rectangles at scroll position zero.
Loaded and failed-bundle states also retain viewport screenshots.

At 1440 pixels, the test also holds and then aborts the actual deferred bundle.
These observations use the real server document and stylesheet with a controlled asset failure.
Pending observations avoid font waits and screenshots until the held request settles.
They do not prove recovery from an external network outage.
The absence of a restart notice is the empty-state control; no placeholder is expected for this synchronous markup.

Test-only baseline `34cf21c9` precedes the production repair.
The repair removes the content column's fixed row so it follows the category menu.
It preserves the mobile single-column override and adds no production lines.
Hosted baseline run `38653244717` and repaired exact-revision evidence are pending.
Local product runtime remains paused by the workstation resource hold.
