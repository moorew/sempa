# AUR — `sempa-bin`

Installs the released Sempa desktop app on Arch/Manjaro from the GitHub `.deb`
(x86_64 + aarch64). The package source of truth is `sempa-bin/PKGBUILD` here;
the AUR git repo mirrors it.

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
