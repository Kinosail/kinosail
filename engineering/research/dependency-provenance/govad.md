# govad source and model provenance verification

Verified 2026-09-06. The selected source/model hashes are enforced by `scripts/quality/check-dependency-integrity.py`. Owner: Subtitles maintainers. Any update requires repeating source review, immutable model reconstruction and reference parity before updating the hashes. Complete reproduction artifacts remain in the local dependency audit bundle.

## Result

The exact embedded govad model was independently reproduced **byte-for-byte** from the official Silero model at an immutable upstream commit. The 337-line Go source was read in full. Its numerical outputs agree with official ONNX inference across both the upstream fixture and an independently generated signal sequence. This materially reduces the opaque-model concern for the selected commit, while not establishing active maintenance or broad adoption.

Selected Go module: `github.com/zserge/govad@v0.0.0-20260330155402-74750eabf3a4`.

Primary sources:

- [Pinned govad source](https://github.com/zserge/govad/blob/74750eabf3a4/vad.go)
- [Pinned govad tree](https://github.com/zserge/govad/tree/74750eabf3a4)
- [Official Silero commit introducing the half model](https://github.com/snakers4/silero-vad/commit/e680ea663339064ddfc6125a98a05f75191fc891), dated 2024-08-22.
- [Exact original ONNX](https://raw.githubusercontent.com/snakers4/silero-vad/e680ea663339064ddfc6125a98a05f75191fc891/src/silero_vad/data/silero_vad_half.onnx)
- [Official MIT license at that commit](https://github.com/snakers4/silero-vad/blob/e680ea663339064ddfc6125a98a05f75191fc891/LICENSE)

The official v5.0 tag resolves through annotated tag `a27ffa1d13db33ee6d9898ee469c6302ccc46912` to commit `5cd2ba54db059f961e7545d4f211b44f005a6dfb`; that tree includes the full ONNX, not the half model. Therefore the verified provenance is the exact August 2024 half-model commit, rather than claiming the file was obtained from the v5.0 tag. GitHub reports that tag unsigned. govad itself describes these weights as Silero VAD v5.

## SHA-256 identities

| Artifact | SHA-256 |
|---|---|
| Exact 337-line `vad.go` | `73fc381fe750e5afc8be27c123682635bc344ac83602784cb834bbd7f939a9b8` |
| Original and independently reproduced `model/silero_vad.bin` | `b8df2e6e32753b7aa47ab59571b0d9d0b490a223f8dc9118bb388efeaec6f8e3` |
| Official pinned `silero_vad_half.onnx` | `1e0b195ad4806595ef4466f419d16fca7e4afcfc6669b8c0b5f76ea87547c769` |

The model contains exactly 309,633 little-endian float32 values, 1,238,532 bytes. Extraction concatenates the named STFT tensor, four convolution weight/bias pairs, recurrent input/hidden weight and bias tensors, then output head weight/bias. No transformation, retraining, quantization, or gate reordering was needed. All initializer names must exactly match the expected allowlist in `verify.py`; the complete output must equal the embedded file.

## Source review

- Imports only `bytes`, `embed`, `encoding/binary`, `fmt`, `io`, `math`, and `os`.
- No network calls, subprocesses, dynamic loading, unsafe, CGo, runtime downloads, telemetry, or shell execution.
- `New()` parses embedded fixed-size float32 tensors. `Process()` performs convolution, magnitude spectrum, ReLU, recurrent gates, and sigmoid using bounded array dimensions.
- Exported `NewFromFile` can read a caller-supplied path, and `NewFromReader` accepts non-finite tensors/trailing bytes. Kinosail does not call these input APIs: `apps/subtitles/internal/server/subtitle_sync.go` calls `govad.New()`, then feeds precisely 512 PCM-derived samples. These API limitations therefore are not an externally controlled model input in the existing integration.
- No application-executed malicious behavior identified in the reviewed source. This is a manual source review, not a proof covering all possible numerical behavior.

The pinned repo mentions `export_for_go.py` in its source but does **not** include that exporter, the original ONNX, a source ONNX checksum, or a pinned original-model commit. Independent reproduction fills that missing provenance for this exact selected artifact. It does not fix upstream documentation or future releases.

## Verification actually run

1. `go test -v .` in an isolated copied module: all three library tests passed, including supplied ONNX reference parity. Full `go test ./...` includes a microphone example requiring undeclared `gen2brain/malgo`; that attempt failed to load the example. No dependency was added for the unused example.
2. Independently downloaded immutable official ONNX, loaded with `load_external_data=False`; validated with `onnx.checker.check_model`. Recursively checked graph/subgraphs use only standard ONNX domains, no local functions, and no initializer external data. No upstream conversion scripts were run.
3. Independently reconstructed all weights and asserted exact byte equality with the selected govad embedded blob.
4. Ran ONNX Runtime CPU inference with one intra-op/inter-op thread, and an independently written Go probe against identical stateful frame sequences:
   - Supplied 20-frame fixture: max Go/official ONNX absolute error **0.000001847743988**; stored upstream reference probabilities equal independently run ONNX probabilities exactly.
   - Independent deterministic 576-frame stream: silence, positive/negative full-scale constant samples, seeded uniform noise, varying tones, seeded quiet Gaussian noise. Max absolute error **0.000001013278961**.
   - Both comfortably below upstream's `0.001` tolerance; outputs finite.

The independent 576-frame fixture SHA-256 is `133214ba71b6f6fac32b9cd04598bf399571d67656f44b7efcbcabc273129df3`. These are parity cases, not a speech-detection accuracy benchmark or a validation of subtitle alignment across languages.

## Reusable artifacts and commands

- `verify.py`: independently authored extraction, graph inspection, fixture generation, and ONNX/Go comparison.
- `govad/cmd/probe/main.go`: independently authored stdin binary-frame Go probe; imports only the locally copied govad package and Go standard library.
- `results.json`: machine-readable hashes, versions, equality, frame counts, errors.
- `requirements-verification.txt`: exact isolated verification environment versions. ONNX, ONNX Runtime and NumPy are temporary verification dependencies only; no production dependency added.
- `reproduced-silero_vad.bin`, `supplied-reference.bin`, `synthetic-reference.bin`, `upstream-tests.log`.

Create a fresh isolated environment in the retained `govad-verification` artifact folder:

```sh
python3 -m venv venv
venv/bin/pip install --only-binary=:all: -r requirements-verification.txt
venv/bin/python verify.py
```

Tested with Python 3.14, numpy 2.5.2, onnx 1.22.0, onnxruntime 1.29.0 on macOS host. No full application suite, Linux/Windows/physical device, or deployed binary validation performed in this subtask. Retain the commit pin, these source/model identities and reproduction script; reassess source and weights whenever the selected module changes. A small-project acceptance is reasonable on this evidence, but its four-star adoption and ongoing maintenance remain separate concerns.
