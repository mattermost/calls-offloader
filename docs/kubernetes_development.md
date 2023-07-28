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
  name: calls-offloader-deployment
  labels:
    app.kubernetes.io/name: calls-offloader
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: calls-offloader
  template:
    metadata:
      labels:
        app.kubernetes.io/name: calls-offloader
    spec:
      serviceAccountName: calls-offloader-service-account
      containers:
      - name: calls-offloader
        image: calls-offloader:dev-d94cee0 # Testing a local image, should point to the official registry when running in prod.
        ports:
        - containerPort: 4545
        env:
          - name: K8S_NAMESPACE # Forwarding the namespace to be used for creation of new resources.
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: DEV_MODE # This is only needed for testing. Should be removed for production use.
            value: "true"
          - name: LOGGER_ENABLEFILE
            value: "false"
          - name: JOBS_APITYPE
            value: "kubernetes"
          - name: JOBS_MAXCONCURRENTJOBS
            value: "1"
          - name: API_SECURITY_ALLOWSELFREGISTRATION # This should only be set to true if running the service inside a private network.
            value: "true"
          - name: LOGGER_CONSOLELEVEL
            value: "DEBUG"
```

We then create the deployment using the YAML file above:

```sh
kubectl apply -f deployment.yaml
```

### Create service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: calls-offloader-service
spec:
  type: NodePort
  selector:
    app.kubernetes.io/name: calls-offloader
  ports:
    - protocol: TCP
      port: 4545
      targetPort: 4545
      nodePort: 30045
```

Finally we create a `NodePort` service to expose the pods to the node:

```sh
kubectl apply -f service.yaml
```

### Verify pods are running correctly

First verify the deployment was created correctly:

```sh
kubectl get deployment -l app.kubernetes.io/name=calls-offloader
```

Then verify the pods are running by checking the logs:

```sh
kubectl logs -l app.kubernetes.io/name=calls-offloader
```

### Expose service URL

Finally we need to expose the service so that it can be accessed from the host.

```sh
minikube service calls-offloader-service --url
```

The returned URL should be used as the value of the *Job service URL* in the Calls configuration settings.

> **_Note_**
>
> The host's IP (e.g. 192.168.49.1) needs to be configured as *ICE Host Override* on the Calls side to get connectivity to calls from within pod to work.
