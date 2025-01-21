### Config Environment Overrides

```
KEY                                            TYPE
API_HTTP_LISTENADDRESS                         String
API_HTTP_TLS_ENABLE                            True or False
API_HTTP_TLS_CERTFILE                          String
API_HTTP_TLS_CERTKEY                           String
API_SECURITY_ENABLEADMIN                       True or False
API_SECURITY_ADMINSECRETKEY                    String
API_SECURITY_ALLOWSELFREGISTRATION             True or False
API_SECURITY_SESSIONCACHE_EXPIRATIONMINUTES    Integer
STORE_DATASOURCE                               String
JOBS_APITYPE                                   JobAPIType
JOBS_MAXCONCURRENTJOBS                         Integer
JOBS_IMAGEREGISTRY                             String
JOBS_KUBERNETES_MAXCONCURRENTJOBS              Integer
JOBS_KUBERNETES_FAILEDJOBSRETENTIONTIME        Duration
JOBS_KUBERNETES_IMAGEREGISTRY                  String
JOBS_KUBERNETES_JOBSRESOURCEREQUIREMENTS       Comma-separated list of Type: pairs
JOBS_KUBERNETES_PERSISTENTVOLUMECLAIMNAME      String
JOBS_KUBERNETES_NODESYSCTLS                    String
JOBS_DOCKER_MAXCONCURRENTJOBS                  Integer
JOBS_DOCKER_FAILEDJOBSRETENTIONTIME            Duration
JOBS_DOCKER_IMAGEREGISTRY                      String
LOGGER_ENABLECONSOLE                           True or False
LOGGER_CONSOLEJSON                             True or False
LOGGER_CONSOLELEVEL                            String
LOGGER_ENABLEFILE                              True or False
LOGGER_FILEJSON                                True or False
LOGGER_FILELEVEL                               String
LOGGER_FILELOCATION                            String
LOGGER_ENABLECOLOR                             True or False
```

### Custom Environment Overrides

```
KEY                                            TYPE
K8S_NAMESPACE                                  String
  The Kubernetes namespace in which jobs will be created.
K8S_JOB_POD_TOLERATIONS                        String (JSON)
  The Kubernetes tolerations to apply to the job pods.
  Example: [{"key":"utilities","operator":"Equal","value":"true","effect":"NoSchedule"}]
```
