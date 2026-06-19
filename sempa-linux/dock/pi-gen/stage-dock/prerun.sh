#!/bin/bash -e
# pi-gen stage hook — start this stage from the previous stage's rootfs.
if [ ! -d "${ROOTFS_DIR}" ]; then
	copy_previous
fi
