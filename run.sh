#!/bin/bash

cd "$(dirname "$0")"
git reset --hard
git clean -fdxq
output="$(build-image . | tee /dev/stderr)"
new_tag="$(echo "$output" | grep "Using image name" | grep -oP "and tags .*" | grep -oP "\"[^\"]+" | grep -oP "[^\"]+" | head -n 1)"
new_image="box-registry.jfrog.io/jenkins/grumpy:$new_tag"
echo "Using image $new_image"
docker tag $new_image 127.0.0.1:5000/grumpy:$new_tag
docker push 127.0.0.1:5000/grumpy:$new_tag

kubectl delete validatingwebhookconfiguration grumpy
kubectl delete mutatingwebhookconfiguration grumpy
kubectl delete pod smooth-app not-smooth-app
kubectl delete secret grumpy
kubectl delete deployment grumpy
kubectl delete service grumpy
kubectl delete pod -l name=grumpy
./gen_certs.sh
kubectl create secret generic grumpy -n default  --from-file=key.pem=certs/grumpy-key.pem  --from-file=cert.pem=certs/grumpy-crt.pem
sed -i "s|pipo02mix/grumpy:1.0.0|localhost:5000/grumpy:$new_tag|g" manifest.yaml
kubectl apply -f  manifest.yaml
kubectl rollout status deployment grumpy
sleep 2
kubectl apply -f app_wrong.yaml
kubectl apply -f app_ok.yaml
echo "'bad' should be changed to 'good'"
kubectl get pod -o json not-smooth-app
