#!/bin/bash -e
# Install the Sempa Dock app + kiosk stack into the image rootfs.
# Drop the aarch64 .deb (from the Linux release) at:
#   stage-dock/01-sempa-dock/files/sempa-dock.deb
# Config files are pulled from this repo's sempa-linux/dock/ tree, assumed copied
# alongside under files/dock/ at image-build time (see README).

FILES="$(dirname "$0")/files"

# 1. The app (the released aarch64 .deb installs /usr/bin/sempa + desktop/icons).
install -m644 "${FILES}/sempa-dock.deb" "${ROOTFS_DIR}/tmp/sempa-dock.deb"
on_chroot << 'EOF'
set -e
apt-get install -y /tmp/sempa-dock.deb || (apt-get -f install -y && apt-get install -y /tmp/sempa-dock.deb)
rm -f /tmp/sempa-dock.deb

# Dedicated unprivileged kiosk user in the seat/video/render groups.
id -u sempa >/dev/null 2>&1 || useradd -m -G video,render,input,seat sempa
EOF

# 2. Kiosk session wrapper + greetd autologin.
install -Dm755 "${FILES}/dock/bin/sempa-dock-session" "${ROOTFS_DIR}/opt/sempa-dock/sempa-dock-session"
install -Dm644 "${FILES}/dock/greetd/config.toml"     "${ROOTFS_DIR}/etc/greetd/config.toml"

# 3. Backlight access for the kiosk user (day/night, idle-dim, tap-to-wake).
install -Dm644 "${FILES}/dock/udev/99-sempa-backlight.rules" "${ROOTFS_DIR}/etc/udev/rules.d/99-sempa-backlight.rules"

# 4. Plymouth boot splash (the breathing mark on terracotta).
mkdir -p "${ROOTFS_DIR}/usr/share/plymouth/themes/sempa"
install -m644 "${FILES}/dock/plymouth/sempa/"* "${ROOTFS_DIR}/usr/share/plymouth/themes/sempa/"

# 5. Quiet/cursor-free boot + Touch Display overlay.
cat "${FILES}/dock/boot/config.txt.snippet" >> "${ROOTFS_DIR}/boot/firmware/config.txt"

on_chroot << 'EOF'
set -e
# Autologin kiosk: greetd owns the session; it respawns on crash (auto-recovery).
systemctl enable greetd
systemctl set-default graphical.target

# Default Plymouth theme → Sempa.
plymouth-set-default-theme sempa || true
update-initramfs -u || true

# Outbound-only firewall; SSH stays off on shipped images (enable key-only if
# you need remote access).
systemctl disable ssh || true
nft -f - << 'NFT' || true
table inet filter {
  chain input  { type filter hook input  priority 0; policy drop;
                 ct state established,related accept; iif "lo" accept; }
  chain output { type filter hook output priority 0; policy accept; }
}
NFT
nft list ruleset > /etc/nftables.conf || true
systemctl enable nftables || true
EOF

echo "Sempa Dock stage installed. Read-only root + overlayfs + the writable /data"
echo "partition are applied by the image post-process (see README)."
