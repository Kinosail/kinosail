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

## Corrected full observation and fixed matrix

b35fcd00a7da0a313ddeaaa2ebc6a638c81f74d1 treee7aa1f22ecd3b196f209c3e3ddfdd535fd97d954.
Corrected public run37748268249 terminal FAILURE, job113214908311.
Receipt69b280d401878709be2812d416f79a587dac70b7839eca4a6f3c335b5796e22d.
Allsix cases complete every stage; source/public native EOF sums exact.
MKV firsttwo source packetDTS missing remain explicit failed clock qualification.
Both12.5 delivery sequences remain480/288..767 vs468/300..767; noframesremoved.
Allsix owned groups unforced joined, twozero observations, peak1, sourceunchanged.
Artifact11537330485 digestfe3dfe15a18d8a294bc22897ad735b512cae7349c8079fb5242e9b6b286f6ba8.

Matrix run37748272351/job113214922827 terminal SUCCESS as diagnosis only.
Receipt e6716cd55659b5072f9276c7d1e486ae597f2ff5406bbc69ebcfe24c185d25cf.
All20 fixed cases observed with complete stages; productionAcceptancefalse.
Artifact11535999228 digest7ec304f50760067a8bd98320c4ed8ffd2e4fb4abcd9fc6e56a721550c6f26580.
Outerdisabled produces12negativeframes; innerdisabled alone doesnotfixcut.
Normalinitialmux + outer/innerdisabled gives videoELSTmediaTime9328/16000,
but stillretains480rawframes including12negative. MP4wholePCM nowexact936008;
MKV935984 vs936000, wholepublicsource-tail starts600016, a16sample defect.
Explicit editlists duplicate normalauto; signedCTS retains10negativeframes,
still480raw. No testedflag combination qualifies requestedrawsequence.
Offline MKV nominalkey12 gives528frames240..767; publicpreparedkey12 gives480,
so offlinekey is not an installed Server argv/source-origin certificate.

The allowed initialmux slice is not admitted by these results. A source-bound
signed/edit/preroll presentation certificate and MKVnative sample precision
remain required before proposing any production acceptance. Existing strategy1
Clock>=0/exact-IDRresume assertions remain untouched.

Independent review found cutoff paths in the diagnostic harness. Child correction
adds one internal shared probe deadline, handledSIGTERM,25sownedcleanup grace,
pre-registersmuxcases, covers Popen/sampler inownedtry, optionalprojectionfields,
and extends only disposable workflow budgets to fit boundedsetup/proof/upload.
28parser/evidence/deadline controls pass in memory; hostedrealcutoff proof
will validate internal expiry and externalSIGTERM with persistedfailedreceipts
and twozero/unforcedowned-groupjoin observations.

Separate workerreuse source diagnosis atmainc452:
startup_preparation.go/startRequest defers cancel after prepareStartupWindow
returnsready; startup_hls.go/watchStartupCancellation cancels only unadoptedjob.
The strict control deliberately waitszeroownedworkers before final2s GET.
Its secondsequential joinedencoder is expected refill, no overlap/leak evidence.
Actualbefore-cancellation adoption needs separate proof and remains with broad
startup lifecycle ownership503/509; no root lifecycle mutation here.
