#!/bin/bash
set -euo pipefail

if [ "$#" -eq 0 ]; then
    EXEC=(nsenter -t 1 -m -u -i -n -p --)
    "${EXEC[@]}" bash "$0" "nsenter"
    exit
fi

# If dpkg is present, install fuse and xfs
if hash dpkg 2>/dev/null; then
    FUSE_INSTALLED=$(dpkg -s fuse | grep Status)
    XFS_INSTALLED=$(dpkg -s xfsprogs | grep Status)

    if [ -z "$FUSE_INSTALLED" ] || [ -z "$XFS_INSTALLED" ]; then
        echo "Installing fuse and xfsprogs"
        apt-get update
        apt-get install -y --no-install-recommends fuse xfsprogs
    fi
else
    >&2 echo "! dpkg unavailable, skipping fuse and xfsprogs install"
fi

# Check if /mnt/codebox already mounted, if not mount it and format
if ! mount | grep -q /mnt/codebox; then
    mkdir -p /mnt/codebox

    if [ -z "$DOCKER_DEVICE" ] || [ -e "$DOCKER_DEVICE" ]; then
        >&2 echo "! docker device not set or does not exist, skipping mounting"
        exit 0
    fi

    if ! hash mkfs.xfs 2>/dev/null; then
        >&2 echo "! mkfs.xfs unavailable, skipping mounting"
        exit 0
    fi

    umount "$DOCKER_DEVICE"
    mkfs.xfs -f "$DOCKER_DEVICE"
    mount -o pquota "$DOCKER_DEVICE" /mnt/codebox
fi
