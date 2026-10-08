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

## First exact-source red proof

Run37746833943 at5d362a946241218a63274600b5b415feb464012f is terminal failure.
Public job113210222092, receiptSHA256
7245abec1435b29b43411629b6179ce06ee81b88e30407f9c9acf4207e3a52f1.
Safe artifact11536700236 ZIPdigest6ded43405f6063c14314196639509340982e468a4d5626ea5ae2e67d338d4262.
Both container12.5 requests reproduce480 frames288..767 vs468 requested.
MP4 native PCM964608 versus936008 reference samples; whole comparison fails.
Missing source MKV DTS caused new diagnostic stage failure; original failures,
raw public frame facts and source/process bounds remain preserved.

Unchanged AAC job113210222307 SUCCESS:470rows/481280samples/exactfullhash as above,
allthree row/PCM dictionaries exact, two coldreopens,375prefix packets, ready.
Original strict prepared-worker result FAILED with audio_required_new_encoder
bothjourneys; two sequential joined starts,peak1,unforcedcoldjoin.
Artifact11535829440 ZIPdigestcf75a22bfee7a5c843203712171d85e890a0b2dbef98ce952f47009e60c6b3c0.

Independent review blocked further diagnostic admission at5d due progressive
fact preservation, cleanup, missing-DTS handling, independent sourcePTS binding,
full EOF accounting and executed-helper checksum/budget gaps.
Child repairs preserve the packet-clock qualification failure, never inferDTS,
save completed stages, use existing owned-session cleanup with twozero samples,
bind sourcePTS, record exact sample accounting and compact/bound complete JSON.
All24 parser/evidence controls passed in memory without local writes or builds.
K packet flag is labelled as such; IDR NAL certification remains separate.

Fixed offline counterfactuals test outer/inner avoid_negative_ts and initial
frag_discont differences in pinned installedFFmpeg. They retain everynegative
frame/packet and parsed unsignedTFDT/signedCTS/ELST. No production acceptance.
Official upstream code: https://github.com/FFmpeg/FFmpeg/blob/n8.1.2/libavformat/hlsenc.c#L800
and https://github.com/FFmpeg/FFmpeg/blob/n8.1.2/libavformat/movenc.c#L6611.
