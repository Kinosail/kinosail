# Non-key source-bound presentation investigation

Base source: c452a2e73cebac42d280247fa9684efb630ecc93 (delivered PR527).
This branch changes diagnostic scripts and its disposable manual workflow only.
No production source, normal CI, scanner, security or assertion relaxations.

Preserved baseline: source 5be75ca8bab7f86d3ff870fb9505828e6e414213,
run37530239319, receipt SHA25653b9e1a778eabd13f282f0d389d94b1003257864c5314f039659514a10518e87.
MKV and copied MP4 input seek12.5 previously yielded480 frames288..767
versus468 independent output-reference frames300..767.
Old branch codex/hls-nonkey-presentation-20261006/cc7e62e remains untouched.

The new public receipt retains all original assertions and each failure,
complete source/public packet rows including payload hashes and side data,
every raw delivered frame with independently measured source PTS,
strict parsed movie/track/edit metadata and unsigned fragment clocks,
native48k stereo PCM without trimming, resampling, padding or time budgets.
Four zero/key container controls are separate observations, not assumed passes.
The non-key public job exits failure unless every strict case passes.

Unchanged accepted AAC control runs separately:470 exact packet rows and
481280 native samples, full hash678541f4dbde6b344ecf2f2d7d754fd46c73341931f68a1d3ddca811864c92b2.
Its historical strict prepared-worker failures stay preserved in the receipt.
A second sequential joined encoder is not evidence of overlap/leak.

Host runner only, pinned Jellyfin FFmpeg8.1.2-5 SHA256
b4e72894ad26c809ed0104805f5415a97be75212b9fcf9d60b89ad25bb3d43e3.
All14 strict parser controls pass before publication, unchanged from retained
parser tests except module names. No native/iOS/Safari/Nox acceptance or manual deployment.
