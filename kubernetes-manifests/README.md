# ./kubernetes-manifests

Plain Kubernetes manifests for the 10 services and `redis-cart`, using the
images published by the upstream project. To deploy the app, use the Kustomize
configuration in [/kustomize](/kustomize) (`kubectl apply -k kustomize/`).
