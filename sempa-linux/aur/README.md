# AUR — `sempa-bin`

Installs the released Sempa desktop app on Arch/Manjaro from the GitHub `.deb`
(x86_64 + aarch64). The package source of truth is `sempa-bin/PKGBUILD` here;
the AUR git repo mirrors it.

> **Status:** the `PKGBUILD` + `.SRCINFO` are ready for 1.2.0 (checksums verified
> against the release). Not yet published — AUR **new-account registration was
> disabled** when we went to publish. Once it reopens, register, add your SSH key,
> and run the publish steps below; the README markets AUR as "coming" until then.

## Publish / update (per release)

```bash
cd sempa-linux/aur/sempa-bin
# 1. Bump pkgver to the new version, reset pkgrel=1.
# 2. Refresh checksums from the uploaded .deb assets:
updpkgsums
# 3. Regenerate the AUR metadata:
makepkg --printsrcinfo > .SRCINFO
# 4. Build-test locally (optional, on Arch):
makepkg -si
# 5. Push to the AUR:
#    (first time) git clone ssh://aur@aur.archlinux.org/sempa-bin.git
git add PKGBUILD .SRCINFO
git commit -m "sempa-bin <version>"
git push aur master
```

Notes:
- `provides`/`conflicts` are `sempa`, so a future from-source `sempa` package
  coexists cleanly.
- The `.deb` payload already carries the desktop entry, hicolor icons, symbolic
  icon and AppStream metainfo (installed under `/usr/share`), so the AUR package
  needs no extra install steps.
