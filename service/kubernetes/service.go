// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mattermost/calls-offloader/public/job"
	"github.com/mattermost/calls-offloader/service/random"

	"github.com/mattermost/mattermost/server/public/shared/mlog"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	k8sDefaultNamespace   = "default"
	k8sJobStopTimeout     = 5 * time.Minute
	k8sRequestTimeout     = 10 * time.Second
	k8sInitContainerImage = "busybox:1.36"
	k8sVolumePath         = "/data"
)

// Type alias and custom decoders to support passing JSON from both TOML config and env
// variable.

type JobsResourceRequirements map[job.Type]corev1.ResourceRequirements

func (r *JobsResourceRequirements) Decode(data string) error {
	return yaml.NewYAMLOrJSONDecoder(bytes.NewBuffer([]byte(data)), 0).Decode(r)
}

func (r *JobsResourceRequirements) UnmarshalTOML(data interface{}) error {
	js, ok := data.(string)
	if !ok {
		return fmt.Errorf("invalid data found")
	}
	return yaml.NewYAMLOrJSONDecoder(bytes.NewBuffer([]byte(js)), 0).Decode(r)
}

type JobServiceConfig struct {
	MaxConcurrentJobs         int
	FailedJobsRetentionTime   time.Duration
	ImageRegistry             string
	JobsResourceRequirements  JobsResourceRequirements `toml:"jobs_resource_requirements"`
	PersistentVolumeClaimName string                   `toml:"persistent_volume_claim_name"`
	NodeSysctls               string                   `toml:"node_sysctls"`
}

func (c JobServiceConfig) IsValid() error {
	if c.MaxConcurrentJobs < 0 {
		return fmt.Errorf("invalid MaxConcurrentJobs value: should be positive")
	}

	if c.FailedJobsRetentionTime > 0 && c.FailedJobsRetentionTime < time.Minute {
		return fmt.Errorf("invalid FailedJobsRetentionTime value: should be at least one minute")
	}

	return nil
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
	if err := cfg.IsValid(s.cfg.ImageRegistry); err != nil {
		return job.Job{}, fmt.Errorf("invalid job config: %w", err)
	}

	if onStopCb == nil {
		return job.Job{}, fmt.Errorf("onStopCb should not be nil")
	}

	devMode := os.Getenv("DEV_MODE") == "true"

	// We fetch the list of jobs to check against it in order to
	// ensure we don't exceed the configured MaxConcurrentJobs limit.
	client := s.cs.BatchV1().Jobs(s.namespace)
	ctx, cancel := context.WithTimeout(context.Background(), k8sRequestTimeout)
	defer cancel()
	jobList, err := client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to list jobs: %w", err)
	}
	if activeJobs := getActiveJobs(jobList.Items); s.cfg.MaxConcurrentJobs > 0 && activeJobs >= s.cfg.MaxConcurrentJobs {
		if !devMode {
			return job.Job{}, fmt.Errorf("max concurrent jobs reached")
		}
		s.log.Warn("max concurrent jobs reached", mlog.Int("number of active jobs", activeJobs),
			mlog.Int("cfg.MaxConcurrentJobs", s.cfg.MaxConcurrentJobs))
	}

	var jobID string
	var jobPrefix string
	var env []corev1.EnvVar
	switch cfg.Type {
	case job.TypeRecording:
		cfg.InputData.SetSiteURL(getSiteURLForJob(cfg.InputData.GetSiteURL()))
		jobPrefix = job.RecordingJobPrefix
		jobID = jobPrefix + "-job-" + random.NewID()
		env = append(env, getEnvFromJobInputData(cfg.InputData)...)
	case job.TypeTranscribing:
		cfg.InputData.SetSiteURL(getSiteURLForJob(cfg.InputData.GetSiteURL()))
		jobPrefix = job.TranscribingJobPrefix
		jobID = jobPrefix + "-job-" + random.NewID()
		env = append(env, getEnvFromJobInputData(cfg.InputData)...)
	}

	var initContainers []corev1.Container
	if s.cfg.NodeSysctls != "" {
		s.log.Info("generating init containers", mlog.String("sysctls", s.cfg.NodeSysctls))
		initContainers, err = genInitContainers(jobID, k8sInitContainerImage, s.cfg.NodeSysctls)
		if err != nil {
			return job.Job{}, fmt.Errorf("failed to generate init containers: %w", err)
		}
	}

	var hostNetwork bool
	if devMode {
		s.log.Info("DEV_MODE enabled, enabling host networking", mlog.String("hostIP", os.Getenv("HOST_IP")))

		// Forward DEV_MODE to recorder process.
		env = append(env, corev1.EnvVar{
			Name:  "DEV_MODE",
			Value: "true",
		})

		// Use local image when running in dev mode.
		cfg.Runner = jobPrefix + ":master"

		// Enable host networking to ease host <--> pod connectivity.
		hostNetwork = true
	}

	tolerations, err := getJobPodTolerations()
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to get job pod tolerations: %w", err)
	}

	var ttlSecondsAfterFinished *int32
	if s.cfg.FailedJobsRetentionTime > 0 {
		ttlSecondsAfterFinished = newInt32(int32(s.cfg.FailedJobsRetentionTime.Seconds()))
	}

	volumes := []corev1.Volume{
		{
			Name: jobID,
		},
	}

	if s.cfg.PersistentVolumeClaimName != "" {
		s.log.Debug("using persistent volume claim", mlog.String("name", s.cfg.PersistentVolumeClaimName))
		volumes[0].VolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: s.cfg.PersistentVolumeClaimName,
			},
		}
	}

	spec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobID,
			Namespace: s.namespace,
			Labels: map[string]string{
				// Using a custom label to easily watch the job.
				"job_name": jobID,
				// app label helps with fetching logs.
				"app": "mattermost-calls-offloader",
			},
		},
		Spec: batchv1.JobSpec{
			// We only support one job at a time and don't want it to
			// restart on failure.
			Parallelism:             newInt32(1),
			Completions:             newInt32(1),
			BackoffLimit:            newInt32(0),
			TTLSecondsAfterFinished: ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						// Using a custom label to easily retrieve the pod later on.
						"job_name": jobID,
						// app label helps with fetching logs.
						"app": "mattermost-calls-offloader",
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: initContainers,
					Containers: []corev1.Container{
						{
							Name:            jobID,
							Image:           cfg.Runner,
							ImagePullPolicy: corev1.PullIfNotPresent,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      jobID,
									MountPath: k8sVolumePath,
								},
							},
							Env:       env,
							Resources: s.cfg.JobsResourceRequirements[cfg.Type],
						},
					},
					Volumes:     volumes,
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
			LabelSelector:  "job_name==" + jobID,
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
	err := client.Delete(ctx, jobID, metav1.DeleteOptions{
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
		LabelSelector: "job_name==" + jobID,
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
