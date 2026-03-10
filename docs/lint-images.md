# lint-images

## Why this exists

Two failure scenarios when image references in `values.yaml` are wrong:

- **Online installs:** tag was never pushed to the registry → pods crash with `ImagePullBackOff`
- **Airgap installs:** `rancher-images.txt` is generated at release time by statically scanning `values.yaml`. The scanner only captures an image when `repository` and `tag` are **sibling keys in the same YAML map node**. A missing or misplaced tag means the image is silently excluded from `rancher-images.txt` — airgap installations fail at runtime with no obvious cause

The previous mitigation was a GitHub Actions workflow that posted a PR comment asking authors to verify image structure manually, gated by a thumbs-up reaction. This was easy to miss and relied on human discipline.

---

## Rules

Three rules enforced on every image block in PR-changed `.tgz` assets:

1. `repository` present + `tag` missing, empty, or null → **block** (`orphan_repository`)
2. `repository` present + `tag` valid + `repository` not `rancher/*` → **block** (`wrong_namespace`)
3. `repository` present + `tag` valid + `repository` is `rancher/*` → **pass**

No exceptions. `appVersion` as a Helm template fallback does not count as a tag.

---

## Scope

- Only scans `.tgz` files **added or modified by the current PR** (git diff against `origin/<base-branch>` merge base)
- Does not scan the full repository

---

## Usage

**CI (GitHub Actions):**
```yaml
- name: Lint image tags
  env:
    BASE_BRANCH: ${{ github.base_ref }}
  run: charts-build-scripts lint-images
```

**Local (bypass git diff):**
```bash
charts-build-scripts lint-images --tgz assets/<chart>/<chart-version>.tgz
```

---

## Key files

| File | Role |
|---|---|
| `pkg/registries/lint.go` | `LintImageTags`, `lintTgz`, `traverseViolations` |
| `pkg/git/gogit.go` | `GetChangedFiles` — go-git merge base tree diff |
| `pkg/registries/assets.go` | `traverseRepoTags` — existing release-time scanner (unchanged) |
| `pkg/filesystem/assets.go` | `DecodeValueYamlInTgz` — tar/gzip decoder |

---

## Edge cases

- `repository: null` — skipped, intentional override slot (e.g. gatekeeper `preInstall.crdRepository`)
- `tag: ""` or `tag: null` — treated as missing, triggers `orphan_repository`
- `repository` behind `enabled: false` with no tag — flagged, see below

### The `enabled: false` anti-pattern

Shipping image references in `values.yaml` behind a disabled feature gate with no resolvable tag is a broken contract:

- `rancher-images.txt` is built at release time from a static scan
- If the tag is missing at release time, the image never enters `rancher-images.txt`
- An airgap customer who later enables the feature will find the image was never mirrored — no recovery path without going back online

**Rule:** do not add image references to `values.yaml` until the feature is ready to ship with a real tag.
