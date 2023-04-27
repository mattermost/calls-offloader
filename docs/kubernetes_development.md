## Kubernetes Development

### Prerequisites

- [docker](https://www.docker.com/) installed and running.
- [minikube](https://minikube.sigs.k8s.io/docs/) installed and running.

### Build local image

#### Load docker environment

We load the minikube docker environment in order to be able to build the docker image on the minikube docker instance:

```sh
eval $(minikube -p minikube docker-env)
```

#### Build docker image

We build the docker image for `calls-offloader` to be used by the Kubernetes deployment:

```sh
make docker-build
```

### Create service account and roles

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: calls-offloader-service-account
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: calls-offloader-role
  namespace: default
rules:
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["get", "list"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: calls-offloader-role-binding
  namespace: default 
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: calls-offloader-role 
subjects:
- namespace: default 
  kind: ServiceAccount
  name: calls-offloader-service-account 
```

We create a `ServiceAccount`, a `Role` and a `RoleBinding` to give `calls-offloader` permissions to create and manage jobs using the above YAML file:

```sh
kubectl apply -f roles.yaml
```

### Create deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: calls-offloader
  labels:
    app: calls
spec:
  replicas: 1
  selector:
    matchLabels:
      app: calls
  template:
    metadata:
      labels:
        app: calls
    spec:
      serviceAccountName: calls-offloader-service-account
      hostNetwork: true # This is needed purely for testing so that it's easier to connect from the local MM instance.
      containers:
      - name: calls-offloader
        image: calls-offloader:master # Testing a local image, should point to the official registry when running in prod.
        ports:
        - containerPort: 4545
        env:
          - name: K8S_NAMESPACE # Forwarding the namespace to be used for creation of new resources.
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: DEV_MODE # This is needed for testing.
            value: "true"
          - name: LOGGER_ENABLEFILE
            value: "false"
          - name: JOBS_APITYPE
            value: "kubernetes"
          - name: API_SECURITY_ALLOWSELFREGISTRATION
            value: "true"
          - name: LOGGER_CONSOLELEVEL
            value: "DEBUG"
```

Finally we create the deployment using the YAML file above:

```sh
kubectl apply -f deployment.yaml
```

### Verify pod is running correctly

To verify the deployment is running correctly:

```sh
kubectl get deployment calls-offloader
```

```sh
kubectl logs -l app=calls
```

### Check pod IP address

Finally we find the IP address for the pod to be used from the Calls side to connect to the service (*Job Service URL*):

```sh
kubectl get pods -l app=calls -o wide
```

> **_Note_**
>
> The host's IP (e.g. 192.168.49.1) needs to be configured as *ICE Host Override* on the Calls side to get connectivity to calls from within pod to work.
