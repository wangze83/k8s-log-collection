#!/bin/bash
wget -q /kubernetes/helm-v3.2.3-linux-amd64.tar.gz

tar -xvf helm-v3.2.3-linux-amd64.tar.gz
mv linux-amd64/helm /usr/bin/

helm plugin install https://github.com/chartmuseum/helm-push


sed -i 's/latest/v0.1.0-${QIXIAO_SCM.GIT_COMMIT[0..6]}/g' log-collection/values.yaml
sed -i 's/latest/${QIXIAO_SCM.GIT_COMMIT[0..6]}/g' log-collection/Chart.yaml
sed -i 's/latest/${QIXIAO_SCM.GIT_COMMIT[0..6]}/g' log-collection/README.md

cat log-collection/values.yaml
cat log-collection/Chart.yaml

helm repo add log-collection https://harbor.wz.net/chartrepo/cloud
helm cm-push log-collection log-collection -u cloud -p CloudHarbor
