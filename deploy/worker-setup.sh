#! /bin/bash
set -euo pipefail

EXEC=(nsenter -t 1 -m -u -i -n -p --)

STARTUP_SCRIPT=' 
FUSE_INSTALLED=$(dpkg -s fuse | grep Status)
XFS_INSTALLED=$(dpkg -s xfsprogs | grep Status)

if [ -z "$FUSE_INSTALLED" ] || [ -z "$XFS_INSTALLED" ]; then
    echo "Installing fuse and xfsprogs"
    apt-get update
    apt-get install fuse xfsprogs
fi

XFS_MOUNTED=$(mount | grep /mnt/codebox)

if [ -z "$XFS_MOUNTED" ]; then
    echo "Preparing and mounting /mnt/codebox"
    mkdir -p /mnt/codebox
    umount $DOCKER_DEVICE
    mkfs.xfs -f $DOCKER_DEVICE
    mount -o pquota $DOCKER_DEVICE /mnt/codebox
fi
'

"${EXEC[@]}" bash -c "${STARTUP_SCRIPT}"
