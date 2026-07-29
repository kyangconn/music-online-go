# Audio analyzer candidates and benchmark protocol

Last reviewed: 2026-07-29

This document records the M5.3 candidate screen, licensing boundary, and reproducible local evaluation protocol. It is not legal advice; the exact image, code revision, and weight artifact must still be reviewed before distribution.

## Decision

Music Online currently bundles, downloads, and enables no audio model by default. Offline metadata rules remain the default, while the HTTP analyzer is an optional adapter.

The first private-library experiment should retain two tracks:

1. A low-cost track: a small AudioSet tagger, preferably EfficientAT and secondarily PANNs/YAMNet, contributes weak vocal/electronic/dubstep/impact evidence alongside local DSP features.
2. An open-vocabulary track: a pinned LAION CLAP artifact scores preset descriptions to determine whether finer labels justify its substantially larger image, memory, and CPU cost.

No model becomes the default, no accuracy is claimed, and no candidate image enters the normal Compose path until it passes a held-out private gold set, the resource budget, and an artifact-level license review.

## Candidate landscape

| Candidate | Strength | Gap for the four presets | Rights and operations conclusion |
|---|---|---|---|
| Metadata rules + DSP | No weights, predictable CPU, and directly explainable evidence | DSP alone cannot reliably identify semantic subgenres such as Chillstep or Complextro | Keep as the default baseline. A sidecar may use a pinned [librosa](https://librosa.org/doc/latest/feature.html) or equivalent implementation; librosa itself is [ISC licensed](https://github.com/librosa/librosa/blob/main/LICENSE.md) |
| [EfficientAT](https://github.com/fschmid56/EfficientAT) | Efficient CNN variants designed for constrained AudioSet inference | AudioSet emphasizes events and broad classes rather than most electronic subgenres | MIT repository; pin and review the exact checkpoint and digest. Preferred lightweight experiment, never a source of truth |
| [PANNs](https://github.com/qiuqiangkong/audioset_tagging_cnn) / [YAMNet](https://www.tensorflow.org/hub/tutorials/yamnet) | Mature 521/527-class AudioSet baselines for voice, music, and some electronic evidence | Still too coarse; older PANNs stacks and larger variants cost more on CPU | PANNs code is MIT, while each Zenodo weight remains a separate artifact to review. Use only as weak evidence |
| [musicnn](https://github.com/jordipons/musicnn) | Music-oriented tags, ISC code, included MTT/MSD models | Its 50-tag vocabularies are coarse and the published package belongs to the TensorFlow 1.x era | Legacy benchmark only; do not place it in the default image |
| [LAION CLAP](https://github.com/LAION-AI/CLAP) | Audio-text similarity permits open preset descriptions without first training a four-class head | Large; official preprocessing uses limited windows, so deterministic multi-segment aggregation is required; zero-shot scores need calibration | Experimental optional candidate only. Pin the exact model card, model SHA-256, and image digest; never infer weight rights from the code license |
| [OpenL3](https://github.com/marl/openl3) | MIT code and CC BY 4.0 weights with a music embedding | Produces embeddings, so a versioned locally trained head is still required | A fallback for a lightweight probe; TensorFlow and libsndfile increase image cost |
| [MERT](https://github.com/yizhilll/MERT), [MusicFM](https://github.com/minzwon/musicfm), [MuQ](https://github.com/tencent-ailab/MuQ) | Music-specific representations useful across tagging, beat, and structure tasks | None is a ready four-preset classifier; large models usually require a trained probe/head and substantial compute | Research candidates. MERT code is Apache-2.0 but some model cards are non-commercial; MuQ weights are explicitly CC BY-NC 4.0; review MusicFM FMA and MSD artifacts separately |
| [Essentia](https://essentia.upf.edu/) and MTG models | Broad MIR/DSP feature coverage and many existing music models | Complex runtime/model matrix and no guarantee of the required fine genre boundaries | The [core licensing](https://essentia.upf.edu/licensing_information.html) and [model licensing](https://essentia.upf.edu/models.html) include AGPL, non-commercial, and commercial-license combinations. Do not bundle; only an operator-supplied, artifact-reviewed adapter may use them |

This table evaluates technical fit only. The `code_license` and `model_license` fields are audit records; the benchmark cannot prove license compatibility.

## Gold-set design

- Label at least 30–50 tracks per preset and include a meaningful `none` negative class.
- Include fusions, long intros, half/double-time material, high-energy instrumentals, and low-energy vocal tracks.
- Keep an artist, album, version, and near-duplicate source in exactly one split. `groups` contains leakage-boundary identifiers rather than descriptive tags; the tool rejects a group that crosses `calibration` and `evaluation`.
- Use `calibration` only for thresholds, weights, prompts, heads, and segment policy. Every reported metric is computed from the untouched `evaluation` split.
- `audio_ref` is a deployment-local relative reference. Never commit copyrighted audio, private absolute paths, or user information.
- Create a new immutable revision when labels change. Candidate results bind the raw manifest bytes by SHA-256.

The checked-in manifest is only a JSON-contract smoke fixture. Both real splits must contain every preset and `none`; the tool rejects missing classes, duplicate IDs, unknown fields, non-finite scores, and incomplete candidate results.

## Candidate result contract

Every result records:

- implementation, model, and rule-bundle versions; a SHA-256 model digest; container candidates use `image@sha256:...`;
- source and weight license identifiers, with `UNKNOWN` treated as not distributable;
- four finite `0..1` scores per successful track, or a stable non-sensitive `error_code`;
- consistently defined process CPU time and peak memory, plus image size and delta measured with the same engine and base image.

Warm each candidate and preferably repeat each track three times, recording median CPU and maximum memory. Segment selection is part of the versioned candidate. Whole-track cheap statistics plus fixed intro/middle/high-energy windows better match this project than intro-only inference.

The report contains per-class precision/recall/F1, macro-F1, high-confidence precision and count, coverage/abstention, failure rate, confusion matrix, mean/p95 CPU, peak memory, and image delta. Candidate runners measure resources; the aggregator does not invent missing measurements.

## Usage

Copy the fixtures and replace them with a local manifest and runner results:

```bash
make benchmark-analysis ARGS="-manifest private/gold-set-v1.json -result private/rules-v1.json -result private/efficientat-v1.json -format markdown"
```

The checked-in fixture verifies schema and rendering only:

```bash
make benchmark-analysis ARGS="-manifest docs/analysis-benchmark/gold-set.example.json -result docs/analysis-benchmark/candidate-result.example.json -format markdown"
```

Its hand-authored scores are not accuracy evidence for any rule or model.

## Production-option gate

1. Record and accept the exact code, model, training-data derivative restrictions, and redistribution rights.
2. Pin image, model, and dependency digests; generate an SBOM and pass vulnerability scanning.
3. Produce a reproducible private evaluation report without tuning prompts, thresholds, or heads on that split.
4. Meet project targets for high-confidence precision, coverage, and per-class recall; do not hide weak classes behind aggregate accuracy.
5. Fit personal/family-server CPU, memory, latency, and image budgets, with a clear non-GPU fallback.
6. Preserve upload, directory import, and playback when the analyzer is offline, slow, or broken.
