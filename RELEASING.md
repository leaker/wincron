# Releasing wincron

How to cut a wincron release and how the Scoop bucket (`leaker/scoop-bucket`)
gets updated automatically. For the maintainer; end users don't need this.

---

## TL;DR — cutting a release

```bash
git tag v0.1.0
git push origin v0.1.0
```

From there everything is automated by `.github/workflows/release.yml`:

1. Builds `wincron.exe` for `windows/amd64` with the version baked in via
   `-ldflags "-X main.version=0.1.0"`.
2. Packages `wincron-0.1.0-windows-amd64.zip` (the exe + README + LICENSE +
   `crontab.example.txt`) and a checksums file.
3. Publishes a GitHub Release with those assets attached.
4. Regenerates `bucket/wincron.json` in `leaker/scoop-bucket` (version, url,
   sha256) and pushes a `wincron: bump to 0.1.0` commit.

End users running `scoop update wincron` resolve to the new version within
seconds. The flow is hands-off after `git push origin v0.1.0`.

> The tag is the single source of truth for the version — there is no
> checked-in version file to keep in sync.

---

## One-time setup: `PACKAGE_MANAGER_BUMP_TOKEN`

The release job pushes a commit into `leaker/scoop-bucket`. The default
`GITHUB_TOKEN` is scoped only to `leaker/wincron`, so a cross-repo push needs a
Personal Access Token stored as a repository secret.

### Option A — fine-grained PAT (least privilege, recommended)

1. <https://github.com/settings/personal-access-tokens/new>
2. Resource owner: `leaker`
3. Repository access: **Only select repositories** → `leaker/scoop-bucket`
4. Repository permissions: **Contents → Read and write** (leave the rest at
   No access)
5. Generate, copy the token once.
6. In `leaker/wincron`: **Settings → Secrets and variables → Actions → New
   repository secret** → name `PACKAGE_MANAGER_BUMP_TOKEN`, paste the value.

Fine-grained tokens max out at 1 year — set a reminder to rotate ~30 days
before expiry.

### Option B — classic PAT (simpler)

Same as above but generate a **classic** token with the **`repo`** scope at
<https://github.com/settings/tokens>, then add it as the same secret.

### Verifying setup

Run the manual backfill against the latest release without cutting a new tag:
**Actions → Bump Scoop manifest → Run workflow** (leave `tag` blank). A
successful run produces a `wincron: bump to X.Y.Z` commit in `leaker/scoop-bucket`
(or prints "nothing to push" if already current).

---

## Manual / backfill bump

`.github/workflows/bump-scoop.yml` (`workflow_dispatch`) re-bumps the manifest
for an existing release. Use it to:

- re-run after rotating an expired PAT,
- re-seed after a hand-edit in the bucket repo,
- point the bucket at a release that predates `release.yml`.

**Actions → Bump Scoop manifest → Run workflow** → enter a tag (`vX.Y.Z`) or
leave blank for the latest published release. It is idempotent — re-running
against an already-bumped tag is a no-op.

---

## Pre-releases (rc / beta / alpha)

Tags like `v0.2.0-rc1` build and publish through the same pipeline and produce
`wincron-0.2.0-rc1-windows-amd64.zip`, but are marked as GitHub **prereleases**
and **skip the Scoop bump** — a pre-release never displaces the stable version
in `scoop update`.

---

## What the release writes to `leaker/scoop-bucket`

`bucket/wincron.json`, regenerated in full each stable release:

| Field                          | Value                                                                 |
|--------------------------------|-----------------------------------------------------------------------|
| `version`                      | tag without the leading `v`                                           |
| `architecture.64bit.url`       | `…/releases/download/vX.Y.Z/wincron-X.Y.Z-windows-amd64.zip`          |
| `architecture.64bit.hash`      | sha256 of that zip                                                    |
| `bin`                          | `wincron.exe`                                                         |
| `checkver` / `autoupdate`      | wired to this repo's releases for Scoop's own update tooling          |

Because the manifest is generated from a template in the workflow, the first
release seeds it and later releases keep it consistent.
