#!/bin/bash
# Run this once as root / with sudo to install the systemd service.
set -e

# Install service file
cp /home/clevercode/aura/deploy/aura.service /etc/systemd/system/aura.service

# Create secrets env file (edit to add GMAIL_CLIENT_ID etc.)
mkdir -p /etc/aura
if [ ! -f /etc/aura/env ]; then
  cat > /etc/aura/env << 'ENVEOF'
# Aura secrets — edit and then: systemctl restart aura
# GMAIL_CLIENT_ID=
# GMAIL_CLIENT_SECRET=
ENVEOF
  chmod 600 /etc/aura/env
fi

systemctl daemon-reload
systemctl enable aura
systemctl start aura
echo "Done. Status:"
systemctl status aura --no-pager
