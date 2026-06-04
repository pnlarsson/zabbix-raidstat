#!/usr/bin/env bash

if [[ $1 = "--detail" ]] && [[ $2 = "--scan" ]]; then cat testdata/mdstat/scan.txt
elif [[ $1 = "--detail" ]] && [[ $2 = "/dev/md0" ]]; then cat testdata/mdstat/md0.txt
elif [[ $1 = "--detail" ]] && [[ $2 = "/dev/md1" ]]; then cat testdata/mdstat/md1.txt
elif [[ $1 = "--detail" ]] && [[ $2 = "/dev/md/2" ]]; then cat testdata/mdstat/md2.txt
elif [[ $1 = "--detail" ]] && [[ $2 = "/dev/md/imsm0" ]]; then cat testdata/mdstat/imsm0.txt
elif [[ $1 = "--detail" ]] && [[ $2 = "/dev/md/Volume0_0" ]]; then cat testdata/mdstat/Volume0_0.txt
fi
