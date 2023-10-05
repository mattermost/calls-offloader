// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package kubernetes

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/mattermost/calls-offloader/public/job"
	"github.com/mattermost/calls-offloader/service/random"

	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"
	"github.com/mattermost/mattermost/server/public/shared/mlog"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	k8sDefaultNamespace   = "default"
	k8sRecordingJobPrefix = "calls-recorder-job-"
	k8sJobStopTimeout     = 5 * time.Minute
	k8sRequestTimeout     = 10 * time.Second
	k8sInitContainerImage = "busybox:1.36"
)

type JobServiceConfig struct {
	MaxConcurrentJobs       int
	FailedJobsRetentionTime time.Duration
}

type JobService struct {
	cfg JobServiceConfig
	log mlog.LoggerIFace

	namespace string
	cs        *k8s.Clientset
}

func NewJobService(log mlog.LoggerIFace, cfg JobServiceConfig) (*JobService, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	cs, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	version, err := cs.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes server version: %w", err)
	}

	namespace := os.Getenv("K8S_NAMESPACE")
	if namespace == "" {
		log.Info("k8s namespace not provided, using default")
		namespace = k8sDefaultNamespace
	}

	log.Info("connected to kubernetes API",
		mlog.String("version", fmt.Sprintf("%s.%s", version.Major, version.Minor)),
		mlog.String("git_version", version.GitVersion),
		mlog.String("namespace", namespace),
	)

	return &JobService{
		cfg:       cfg,
		log:       log,
		cs:        cs,
		namespace: namespace,
	}, nil
}

func (s *JobService) Init(_ job.ServiceConfig) error {
	// May be best not to mess with k8s image pulling policy for now.
	// It's probably okay for images to be pulled upon first pod execution.
	// In the future we may consider executing a dry-run job on start for the purpose of
	// preloading the image.
	return nil
}

func (s *JobService) CreateJob(cfg job.Config, onStopCb job.StopCb) (job.Job, error) {
	if onStopCb == nil {
		return job.Job{}, fmt.Errorf("onStopCb should not be nil")
	}

	if cfg.Type != job.TypeRecording {
		return job.Job{}, fmt.Errorf("job type %s is not implemented", cfg.Type)
	}

	jobID := random.NewID()
	jobName := k8sRecordingJobPrefix + jobID

	var recCfg recorder.RecorderConfig
	recCfg.FromMap(cfg.InputData)
	recCfg.SetDefaults()

	var env []corev1.EnvVar
	var hostNetwork bool

	devMode := os.Getenv("DEV_MODE") == "true"

	client := s.cs.BatchV1().Jobs(s.namespace)
	ctx, cancel := context.WithTimeout(context.Background(), k8sRequestTimeout)
	defer cancel()

	// We fetch the list of jobs to check against it in order to
	// ensure we don't exceed the configured MaxConcurrentJobs limit.
	jobList, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to list jobs: %w", err)
	}
	if activeJobs := getActiveJobs(jobList.Items); activeJobs >= s.cfg.MaxConcurrentJobs {
		if !devMode {
			return job.Job{}, fmt.Errorf("max concurrent jobs reached")
		}
		s.log.Warn("max concurrent jobs reached", mlog.Int("number of active jobs", activeJobs),
			mlog.Int("cfg.MaxConcurrentJobs", s.cfg.MaxConcurrentJobs))
	}

	if devMode {
		s.log.Info("DEV_MODE enabled, enabling host networking", mlog.String("hostIP", os.Getenv("HOST_IP")))

		// Forward DEV_MODE to recorder process.
		env = append(env, corev1.EnvVar{
			Name:  "DEV_MODE",
			Value: "true",
		})

		// Use local image when running in dev mode.
		cfg.Runner = "calls-recorder:master"

		// Override the siteURL hostname with the alias so that the recorder can
		// connect.
		u, err := url.Parse(recCfg.SiteURL)
		if err != nil {
			s.log.Warn("failed to parse SiteURL", mlog.Err(err))
		} else if u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" {
			u.Host = "host.minikube.internal" + ":" + u.Port()
			recCfg.SiteURL = u.String()
		}

		// Enable host networking to ease host <--> pod connectivity.
		hostNetwork = true
	}

	env = append(env, getEnvFromConfig(recCfg)...)

	tolerations, err := getJobPodTolerations()
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to get job pod tolerations: %w", err)
	}

	var ttlSecondsAfterFinished *int32
	if s.cfg.FailedJobsRetentionTime > 0 {
		ttlSecondsAfterFinished = newInt32(int32(s.cfg.FailedJobsRetentionTime.Seconds()))
	}

	spec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: s.namespace,
			Labels: map[string]string{
				// Using a custom label to easily watch the job.
				"job_name": jobName,
				// app label helps with fetching logs.
				"app": "mattermost-calls-offloader",
			},
		},
		Spec: batchv1.JobSpec{
			// We only support one recording job at a time and don't want it to
			// restart on failure.
			Parallelism:             newInt32(1),
			Completions:             newInt32(1),
			BackoffLimit:            newInt32(0),
			TTLSecondsAfterFinished: ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						// Using a custom label to easily retrieve the pod later on.
						"job_name": jobName,
						// app label helps with fetching logs.
						"app": "mattermost-calls-offloader",
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:            jobName + "-init",
							Image:           k8sInitContainerImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								// Enabling the `kernel.unprivileged_userns_clone` sysctl at node level is necessary in order to run Chromium sandbox.
								// See https://developer.chrome.com/docs/puppeteer/troubleshooting/#recommended-enable-user-namespace-cloning for details.
								"sysctl",
								"-w",
								"kernel.unprivileged_userns_clone=1",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: newBool(true),
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            jobName,
							Image:           cfg.Runner,
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      jobName,
									MountPath: "/recs",
								},
							},
							Env: env,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: jobName,
						},
					},
					Tolerations: tolerations,
					// We don't want to ever restart pods as any failure needs to be
					// surfaced to the user who should hit record again.
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: newInt64(int64(k8sJobStopTimeout.Seconds())),
					// ActiveDeadlineSeconds will mark the pod and job as failed without
					// actually deleting it.
					ActiveDeadlineSeconds: newInt64(cfg.MaxDurationSec),
					// HostNetwork should only be used for local testing purposes when DEV_MODE env
					// var is set.
					HostNetwork: hostNetwork,
				},
			},
		},
	}

	ctx, cancel = context.WithTimeout(context.Background(), k8sRequestTimeout)
	defer cancel()

	if _, err := client.Create(ctx, spec, metav1.CreateOptions{}); err != nil {
		return job.Job{}, fmt.Errorf("failed to create job: %w", err)
	}

	jb := job.Job{
		ID:      jobID,
		StartAt: time.Now().UnixMilli(),
		Config:  cfg,
	}

	// We wait for the job to complete to cover both the case of unexpected error or
	// the execution reaching the configured MaxDurationSec. The provided callback is used
	// to update the caller about this occurrence.
	go func() {
		timeoutSecs := cfg.MaxDurationSec + int64(k8sJobStopTimeout.Seconds())

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
		defer cancel()
		watcher, err := client.Watch(ctx, metav1.ListOptions{
			Watch:          true,
			TimeoutSeconds: newInt64(timeoutSecs),
			LabelSelector:  "job_name==" + jobName,
		})
		if err != nil {
			s.log.Error("failed to watch job", mlog.Err(err))
			return
		}
		defer watcher.Stop()

		var success bool
		for ev := range watcher.ResultChan() {
			jb, ok := ev.Object.(*batchv1.Job)
			if !ok {
				continue
			}

			s.log.Debug("job event", mlog.String("jobID", jobID), mlog.Any("type", ev.Type))

			if jb.Status.Failed > 0 {
				s.log.Error("job failed", mlog.String("jobID", jobID))
				break
			}

			if jb.Status.Succeeded > 0 {
				s.log.Info("job succeeded", mlog.String("jobID", jobID))
				success = true
				break
			}

			if ev.Type == watch.Deleted {
				s.log.Info("job was deleted", mlog.String("jobID", jobID))
				return
			}
		}

		if err := onStopCb(jb, success); err != nil {
			s.log.Error("failed to run onStopCb", mlog.Err(err), mlog.String("jobID", jb.ID))
		}

		s.log.Info("watcher done", mlog.String("jobID", jobID))
	}()

	return jb, nil
}

func (s *JobService) DeleteJob(jobID string) error {
	client := s.cs.BatchV1().Jobs(s.namespace)
	ctx, cancel := context.WithTimeout(context.Background(), k8sRequestTimeout)
	defer cancel()

	// Setting propagationPolicy to "Background" so that pods
	// are deleted as well when deleting a corresponding job.
	propagationPolicy := metav1.DeletePropagationBackground
	err := client.Delete(ctx, k8sRecordingJobPrefix+jobID, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	return nil
}

func (s *JobService) GetJobLogs(jobID string, _, stderr io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), k8sRequestTimeout)
	defer cancel()

	list, err := s.cs.CoreV1().Pods(s.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "job_name==" + k8sRecordingJobPrefix + jobID,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods for job: %w", err)
	}

	if len(list.Items) == 0 {
		return fmt.Errorf("no pods found")
	}

	// TODO: consider supporting multiple pods per job.
	pod := list.Items[0]

	var opts corev1.PodLogOptions
	req := s.cs.CoreV1().Pods(s.namespace).GetLogs(pod.Name, &opts)

	podLogs, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer podLogs.Close()

	if _, err := io.Copy(stderr, podLogs); err != nil {
		return fmt.Errorf("failed to copy data from stream: %w", err)
	}

	return nil
}

func (s *JobService) Shutdown() error {
	return nil
}
