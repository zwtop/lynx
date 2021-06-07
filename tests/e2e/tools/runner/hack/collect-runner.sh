#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o xtrace
set -o nounset

mnt_log_dir="${1}"

mkdir -p ${mnt_log_dir}/{${ELF01HOST},${ELF02HOST},${ELF03HOST}}

scp -o StrictHostKeyChecking=no ${ELF01HOST}:/var/log/lynx*.log ${mnt_log_dir}/${ELF01HOST}/
scp -o StrictHostKeyChecking=no ${ELF02HOST}:/var/log/lynx*.log ${mnt_log_dir}/${ELF02HOST}/
scp -o StrictHostKeyChecking=no ${ELF03HOST}:/var/log/lynx*.log ${mnt_log_dir}/${ELF03HOST}/

chmod 0755 ${mnt_log_dir}/{${ELF01HOST},${ELF02HOST},${ELF03HOST}}/*
