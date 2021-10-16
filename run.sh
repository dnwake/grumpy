#!/bin/bash

cd "$(dirname "$0")"
git reset --hard
kubectl delete validatingwebhookconfiguration grumpy
kubectl delete pod smooth-app not-smooth-app
kubectl delete secret grumpy
kubectl delete deployment grumpy
kubectl delete service grumpy
kubectl delete pod -l name=grumpy
./gen_certs.sh
kubectl create secret generic grumpy -n default  --from-file=key.pem=certs/grumpy-key.pem  --from-file=cert.pem=certs/grumpy-crt.pem
kubectl apply -f  manifest.yaml
kubectl rollout status deployment grumpy
echo "This should fail"
kubectl apply -f app_wrong.yaml
echo "This should succeed"
kubectl apply -f app_ok.yaml 
