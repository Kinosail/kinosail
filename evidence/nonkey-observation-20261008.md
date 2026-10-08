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

Cutoff source cfaf7641/tree7f12ba336a736ac49f1dde3723b9475a09eb6212 independently
admitted for bounded hosted testing, productionfalse. Run37749806634 controls28
passed but realfixtureFAILED beforeownedprocess creation: child reentry found
parent evidence directory alreadyexists; no childreceipt, parentFileNotFoundError.
Job113219975381 logSHA8699e5b413a6db6f18338282d933d6d82c2072b1252fe867dcb90bcff473ca87.
Child setup now permits its own existing isolated directory; failed attempt stays.
MKV16sample result is a start-boundary discrepancy, not provenaudibleloss or EOFdefect.

Cutoff-only child531eb3eb passed hosted run37750654595/job113222768733. Internal deadline and actual timeout SIGTERM preserved explicit bounded_diagnostic_deadline failures, expected exits1/124, unforced owned joins, two zero samples, no remaining PIDs/cleanup/qualification failures. Artifact11537548626/1276bytes ZIPsha256c274ac5af815ac184cce9c547cf5fd25f291fc5aa5cc3720197f373b5c21748a; UTF8connector logSHA67bffab72c4c5f758e26ac28fd25667dd546eccccbb9955d813c7c359f428ef0/27724bytes. Previous cfaf failure remains.

Next boundary-only child retains all raw/default-decoded frames and complete packets/PCM. Four fixed legacy/normal-both MKV/bitcopy-MP4 cases report strict physical-fragment-to-demux edit clock correspondence, independent ignore_editlist counterfactual, explicit packet discard flags, separate unqualified nonnegative frame-clock subset, cumulative native source sample clocks, and exact copied AAC payload tails. This is a diagnosis, not a renderer/preroll certificate or production admission. No production/normalCI/security/gate/source oracle changes; original49x/503 branches untouched.

Independent pre-dispatch review of968 caught existing stream_metadata omitting track IDs and incomplete ignore-edit row retention. A dedicated bounded ffprobe index/id/codec_type/time_base probe plus required-field/duplicate-identity controls replace that assumption. Complete probe, ignore-edit packet and native source-frame rows are preserved before interpretation. No production or historical helper modified.

Final review requires ignore-edit packets and explicit stream ID/slot/codec/timebase from the same independent probe. The diagnostic retains and validates those rows, proves unchanged physical track identity, and binds against that independent probe clock rather than default-demux metadata.

Boundary source `fd9499672312dfdef7dd63580857f091ab432b60`, tree `884f106748af8ac5a622d86f21df2d6d9dcf36d8`, was independently admitted. Hosted run [37752435335](https://github.com/Kinosail/kinosail/actions/runs/37752435335), job113228689642, completed all four cases/all five stages with no observation failures and 41 passing controls. Receipt SHA256 `8b41e6c2ab8fa445aa856d907f427c1d431a032f2ca5384a03a3c243cdc1a75b`; artifact11538910438/2904332bytes, ZIP SHA256 `a9da94fdcefb54b74b9974305049c8039263e0d4ff959686b4822288fa8c540b`. UTF8 connector log SHA256 `a5de9aba50615bcf9383911cc4ad1170cf9c134c7f3e04ce74f32be9b42805fa`/74265bytes. Default and ignore-edit packet clocks exactly match raw physical clocks with and without the parsed edit, no discontinuities or packet discard flags. Normal-both retains all480 frames; the separately reported negative source288..299 and nonnegative468/300..767 subsets remain unqualified for presentation acceptance. MKV whole PCM is a unique complete source tail from600016/reference unequal; MP4 from600008/reference equal, with -8 source ordinal/PTS residual. Original init/fragments unchanged; no production admission.

Next fixed generated-copy AAC edit-field diagnosis retains original init/fragments and all packet payloads/video rows. It compares MKV mediaTime deltas -16/-15 and MP4 -1/+1 against full independently decoded references; MP4's nominal600008 reference correspondence remains accepted for this diagnosis. Byte-level assertions permit only the selected AAC ELST mediaTime field change; all other init fields/bytes are unchanged, every native sample is retained, complete EOF is required, and every counterfactual failure stays recorded. This tests causality only; production/audio phase certification and renderer/discard mapping remain open.

Pre-dispatch review of the generated-edit driver caught a false payload failure: changing only an audio clock can legitimately change cross-track demux interleaving. Complete global packet rows and global equality remain recorded; exact per-stream packet order/count/hash assertions protect unchanged payloads. New controls require inter-track swaps to report global inequality while per-track identity passes, and same-track reorder/omission or unknown streams to fail. Historical public packet assertions remain unchanged.

Fixed-field causal source `eb7e82583d42d8e40511cf8d21de2c1e7529ae12`, tree `8a0e2f73d1d021614f03f311f368f418a92757fb`, was independently admitted. Hosted run [37754969565](https://github.com/Kinosail/kinosail/actions/runs/37754969565), job113237172550, terminal SUCCESS at exact source: all six cases observed, all 52 controls passed. Receipt SHA256 `d280e607eac6f4dfac901809cae5fec4b0009a79f6b633afa37d2dcc662014e6`; artifact11538864963/1781507bytes, ZIP SHA256 `51e7082e5b1db46c276ba5ea280dbed24a1d083eecf65b6166e8f1db8bc8a574`; UTF8connector log SHA256 `a2362a3e19fa76fd653ff146d4aa476bcbcb7e26fab20dd3e985bead70406971`/58555bytes. MKV generated ELST27600→27584 alone gives936000 samples/PCM SHA256 `cc611ac491dd0407bddb3db9985c802f2f4401dae66d3d3ade269f06b1b72901`, exact full reference. Adjacent27585 gives935999/not equal. MP4 original28600/reference936008 matches;28599 and28601 give936009/936007, neither equal. All four modifications retain complete per-track packet order/count/hash and complete original video rows, source/init/fragments unchanged, complete native EOF. MKV cross-track interleaving actually changes and remains separately recorded. Independent review qualifies this fixed-fixture causal result only.

The executed fixed-field safe projection reported rawFrames0 for modified copies because its selector read only original observations. CompleteVideoRows remain in the full receipt and an executed strict guard requires equality with all480 original rows. A presentation-only selector correction now reports that retained row count and inherited exact-requested-sequence failure; historical receipt/logs are immutable.

Next source-derived generated-copy proof uses two fixed further seek positions13.5 and18.2 in both containers. It finds a unique native source-frame interval containing the requested48k timestamp, derives desired native ordinal from that frame's measured PTS/ordinal residual, and subtracts the exact first-copied native ordinal to derive audio ELST mediaTime. Reference PCM is not used to select that value. Derived±32 bounds and an independently declared adjacent one-sample negative control are preserved. Unsupported clock ambiguity/phase/codec/source origins remain unqualified. This extends diagnostic explanation, not production acceptance.

Source-derived local-frame model `18459ee80a7e2764b43b11607583df4f434dc05f`, tree `2564579085ddce82ebc6c91d0692118d3b7c24d9`, retained all12 cases in hosted [37757058525](https://github.com/Kinosail/kinosail/actions/runs/37757058525), job113244104033. All58 controls passed. Receipt SHA256 `2198105ea6e66b1680d38a1ccf5ef1ec7238407d847b2fed185a41c09d32f002`; artifact11540687290/3166492bytes, ZIP SHA256 `780271a5121db76f61887147dbe7774503965a0eeee2b8362913611faacc6587`; UTF8connector log SHA256 `510bbe2379252ab3a8dcad1f21bc6d5984c477690bb5b0f0a7b0af2974b50b40`/74845bytes. All generated copies preserve per-track payloads/video/source/assets/EOF. The model is DISPROVED for MKV: at13.5 delta-32 gives888016 versus888000reference (original887984); at18.2 delta+16 gives662384 versus662400reference (originalalreadyexact). MP4 delta0 matches both references; one-sample controls fail. Three declared hypotheses fail. Workflow SUCCESS means preserved observations, not model acceptance. Independent review admits this as counterevidence only. Raw video remains480/336frames and requested-sequencefalse.

Pinned [FFmpeg audio_ts_process](https://github.com/FFmpeg/FFmpeg/blob/n8.1.2/fftools/ffmpeg_dec.c) calls stateful [av_rescale_delta](https://github.com/FFmpeg/FFmpeg/blob/n8.1.2/libavutil/mathematics.c) before user filtering. Raw ffprobe timestamps do not include that CLI normalization. Source-supported mechanism remains an inference until actual clock measurement. In [configure_output_audio_filter](https://github.com/FFmpeg/FFmpeg/blob/n8.1.2/fftools/ffmpeg_filter.c), user ashowinfo precedes automatic output-seek atrim; post-trim PCM framemd5 has separate output framing and clock.

Next clock-only diagnostic retains complete raw-native and pre-trim user-filter rows, independently binds every PCM encoder packet to complete native/reference PCM byte extents and MD5s, and reports source-tail/sampleclock correspondence for unseeked/12.5/13.5/18.2 cases in both fixed containers. It selects no new timestamp edit/model. Original failures, control, source assets, production and normal CI remain unchanged. Bounds240s internal/300s outer/32MiB receipt/4096rows/300physical lines remain.

Independent pre-dispatch review of625830 caught the unchanged stream_metadata helper omitting audio rate/channels. The clock-only driver now uses the existing bounded strict audio_stream probe and retains that exact audio schema separately, with progressive source setup facts and unchanged source binding. Shared helpers/oracles remain unchanged;625830 was never dispatched.

Exact replacement review additionally requires a successfully observed unseeked baseline before any seeked case can assert identical pre-trim sequence. Output command digests and verified packet/PCM rows are now assigned progressively before filter parsing, preserving earlier qualified facts on a later parser failure. No dispatch occurred at625830 or467c9c0b.
