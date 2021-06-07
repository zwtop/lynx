#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o xtrace
set -o nounset

## start build and deploy on elf
builder_dir="$(mktemp -d)/lynx-e2e-builder"
lynx_repo_dir="${builder_dir}/lynx"
lynx_bin_dir="${builder_dir}/bin"
remote_repo="${1}"
remote_refspec="${2}"

git init ${lynx_repo_dir}
git -C ${lynx_repo_dir} remote add origin ${remote_repo}
git -C ${lynx_repo_dir} fetch origin --depth=1 ${remote_refspec}:e2e-runner
git -C ${lynx_repo_dir} checkout e2e-runner

### show commit id
printf "GIT_COMMIT_ID=%s\n" "$(git -C ${lynx_repo_dir} log -1 --pretty=format:"%H")"

cd ${lynx_repo_dir}
timeout 10m go mod download
CGO_ENABLED=0 go build -o ${lynx_bin_dir}/lynx-controller ${lynx_repo_dir}/cmd/lynx-controller/*.go
CGO_ENABLED=0 go build -o ${lynx_bin_dir}/lynx-agent ${lynx_repo_dir}/cmd/lynx-agent/*.go
CGO_ENABLED=0 go build -o ${lynx_bin_dir}/lynx-plugin-tower ${lynx_repo_dir}/plugin/tower/cmd/*.go

ssh -o StrictHostKeyChecking=no ${ELF01HOST} systemctl restart kube-apiserver etcd
ssh -o StrictHostKeyChecking=no ${ELF02HOST} systemctl restart kube-apiserver etcd
ssh -o StrictHostKeyChecking=no ${ELF03HOST} systemctl restart kube-apiserver etcd

sleep 5s # wait for apiserver ready
kubectl apply -f ${lynx_repo_dir}/deploy/crds/

ssh -o StrictHostKeyChecking=no ${ELF01HOST} systemctl stop lynx-controller lynx-agent lynx-plugin-tower
ssh -o StrictHostKeyChecking=no ${ELF02HOST} systemctl stop lynx-controller lynx-agent
ssh -o StrictHostKeyChecking=no ${ELF03HOST} systemctl stop lynx-controller lynx-agent

scp -o StrictHostKeyChecking=no ${lynx_bin_dir}/* ${ELF01HOST}:/usr/local/bin/
scp -o StrictHostKeyChecking=no ${lynx_bin_dir}/* ${ELF02HOST}:/usr/local/bin/
scp -o StrictHostKeyChecking=no ${lynx_bin_dir}/* ${ELF03HOST}:/usr/local/bin/

ssh -o StrictHostKeyChecking=no ${ELF01HOST} systemctl restart lynx-controller lynx-agent lynx-plugin-tower
ssh -o StrictHostKeyChecking=no ${ELF02HOST} systemctl restart lynx-controller lynx-agent
ssh -o StrictHostKeyChecking=no ${ELF03HOST} systemctl restart lynx-controller lynx-agent

# build go test into static file
CGO_ENABLED=0 go test -o /usr/local/bin/e2e.test -c ${lynx_repo_dir}/tests/e2e/cases/*.go
