# R18 document identity recovery

The current browser adapter shares one localStorage target across simultaneous
documents. The server claim and release prerequisites are delivered by PR522.
This separate recovery starts from PR523 main `63f38a09290b99d5a0eb80f2e35bfe7c7b879a60`.
Original PR498 and its source and historical evidence remain preserved.

The first checkpoint changes only tests, fixture preparation and required browser
selection. No production behavior has changed and no runtime result is claimed.
Three new hosted journeys require real simultaneous targets, an addressed public
seek with an untouched sibling, and stable reload identity without stored claims.
All existing selected journeys remain required.

The previously approved fictional clip is copied without re-encoding. Preparation
rejects a different size or SHA-256 before creating a new disposable directory.
The two copies are distinct Library items with exactly the same approved bytes.
Actual browser readiness is required; no mocked media or state response is used.

Failure analysis before implementation:

- Simultaneous or copied-storage documents may alias a target or drain commands.
- Reload, history or lost release may churn candidates or reuse retired authority.
- Occupied targets, hung requests or unavailable storage may wait without a bound.
- Poll/SSE overlap may reorder command effects or consume a sibling's command.
- Pagehide and BFCache may leak authority, listeners, timers or delayed responses.
- Authenticated Profile changes may apply an old response to the new document.
- Malformed or oversized responses may reach playback side effects.
- Tokens may enter persisted storage, diagnostics or public evidence.
- Served asset, fixture or source drift may be misreported as current acceptance.

The later client slice owns only a new identity helper, the existing Home Assistant
adapter block and the helper prepend. Native intent, preparation, progress, queue,
navigation and backend HLS source stay intact. Tokens belong to document memory;
a Profile-scoped sessionStorage ID is only a reload candidate. Server authority,
30-second expiry and no takeover stay authoritative. Reconnect waits at most
35 seconds. Fresh-navigation live conflict may fork because copied storage and
lost release cannot be distinguished. Denied storage permits an ephemeral target
and cannot promise reload identity. No browser fingerprint or physical-device
lifetime is introduced.

Actual Profile switching, storage denial, non-persisted history, lost release and
simultaneous commands require current-source public/browser proof. Virtual-clock
deadline and malformed/delayed reply tests protect faults the real server cannot
produce; they are isolated checks. Actual persisted BFCache and ordinary HTTP
remain separate admission boundaries. Source-only proof cannot satisfy them.
Final independent review, both served asset hashes, strict protected checks,
fetched main ancestry and automatic publication are required before delivery.

The next written-first checkpoint adds isolated malformed-response, bounded-reader,
SSE coalescing, retired-document and private diagnostic controls. It extracts the
existing real-document test setup unchanged. The identity helper remains absent;
these controls have not run and must receive a bounded hosted invocation later.
The first test-only hosted baseline remains pinned to `b28aba970f669ce2d0d5c329eab48ae7cb58a13b`.
The existing media presentation clears its source on pagehide, and the queue closes
its state. Actual playable persisted BFCache needs that owner's disposition;
virtual identity re-entry cannot establish media restoration.
