## Performance

### Calls Recordings

#### Quality profiles

Recording quality can be configured through the [Calls plugin](https://docs.mattermost.com/configure/plugins-configuration-settings.html#call-recording-quality). At this time we provide the following quality profiles:

| Profile | Resolution | Framerate | Bitrate                          |
|--------:|-----------:|----------:|---------------------------------:|
| Low     | 720p       | 15fps     | 1Mbps (video) / 64Kbps (audio)   |
| Medium  | 720p       | 20fps     | 1.5Mbps (video) / 64Kbps (audio) |
| High    | 1080p      | 20fps     | 2.5Mbps (video) / 64Kbps (audio) |

#### Benchmarks

These are the results of a series of benchmarks that were conducted to verify the scalability capabilities of the service. All tests were executed on a AWS EC2 `c6i.2xlarge` instance which is the recommended instance class and size (`8vCPU / 16GB RAM`) for Calls recordings:

| Profile | Concurrency | CPU (avg) | Memory (avg) | Recording size (avg) |
|--------:|------------:|----------:|-------------:|---------------------:|
| Low     | 8           | 66%       | 4GB          | 0.5GB/hour           |
| Medium  | 6           | 66%       | 4GB          | 0.7GB/hour           |
| High    | 4           | 72%       | 4GB          | 1.2GB/hour           |

We recommend setting the [`max_concurrent_jobs`](https://github.com/mattermost/calls-offloader/blob/85717457b3e699fd507e8bed4586e82daa19a045/config/config.sample.toml#L33) config option to the values above, based on the quality profile used.

On the Mattermost side, it may also be necessary to tune the [`FileSettings.MaxFileSize`](https://docs.mattermost.com/configure/environment-configuration-settings.html#maximum-file-size) setting depending on the profile chosen and the configured [`MaxCallDuration`](https://docs.mattermost.com/configure/plugins-configuration-settings.html#maximum-call-recording-duration).

> **_Note_** 
> If a load-balancer or proxy is in front of Mattermost, extra configuration may be necessary. 
> As an example, `nginx` would likely require `client_max_body_size` to be set accordingly.

### Calls Transcriptions

#### Deployment

- `calls-offloader` `v0.5.0`
- `calls-transcriber` `v0.1.0`
- `c6i.2xlarge` EC2 instance
	- 8vCPU / 16GB RAM

#### Model sizes

The transcriber's model size can be configured through the [Calls plugin](https://docs.mattermost.com/configure/plugins-configuration-settings.html#transcriber-model-size). At this time we support the following [Whisper.cpp](https://huggingface.co/ggerganov/whisper.cpp) models:

| Model | File size | Memory |
|------:|----------:|-------:|
| tiny  | 75MB      |~273MB  |
| base  | 142MB     |~388MB  |
| small | 466MB     |~852MB  |

#### Benchmarks

| Model | Threads | CPU (avg) | Memory (avg) | Call duration | Processing time |
|-------|---------|-----------|--------------|---------------|-----------------|
| tiny  | 1       | 13.5%     | 1.20GB       | 10m           | 2m20s (4.28x)   |
| base  | 1       | 13.0%     | 1.23GB       | 10m           | 4m45s (2.10x)   |
| small | 1       | 12.8%     | 1.67GB       | 10m           | 16m50s (0.59x)  |
| tiny  | 2       | 25.0%     | 1.20GB       | 10m           | 1m17s (7.79x)   |
| base  | 2       | 25.5%     | 1.27GB       | 10m           | 2m41s (3.73x)   |
| small | 2       | 25.3%     | 1.68GB       | 10m           | 9m23s (1.07x)   |
| tiny  | 4       | 49.4%     | 1.20GB       | 10m           | 45s (13.33x)    |
| base  | 4       | 49.8%     | 1.27GB       | 10m           | 1m32s (6.52x)   |
| small | 4       | 49.6%     | 1.71GB       | 10m           | 5m27s (1.84x)   |
| tiny  | 4       | 48.7%     | 1.85GB       | 60m           | 3m38s (16.51x)  |
| base  | 4       | 49.5%     | 1.70GB       | 60m           | 7m6s (8.45x)    |
| small | 4       | 50.0%     | 1.99GB       | 60m           | 22m50s (2.63x)  |

### Calls Live Captions

#### Deployment

- `calls-offloader` `v0.8.0`
- `calls-transcriber` `v0.2.2`
- `c6i.2xlarge` EC2 instance
	- 8vCPU / 16GB RAM

#### Model sizes

The live captions model size can be configured through the [Calls plugin](https://docs.mattermost.com/configure/plugins-configuration-settings.html#live-captions-model-size). At this time we support the following [Whisper.cpp](https://huggingface.co/ggerganov/whisper.cpp) models:

| Model | File size | Memory |
|------:|----------:|-------:|
| tiny  | 75MB      |~273MB  |
| base  | 142MB     |~388MB  |
| small | 466MB     |~852MB  |

#### Benchmarks

Legend:
- < Xs: the percentage of audio chunks that were processed within X seconds. E.g., a 90% for < 2s means 90% of all the audio chunks were processed within 2s. When audio is processed within 2s or 4s, the captions feel "real time." When it takes 6s or longer, the audio feels "laggy."
- Buffer full: The live captioning system has a buffer for chunks of audio. When the buffer is full it will drop older chunks of audio so that it is spending time captioning newer audio. A larger number of buffer full events indicates the captioning system is having trouble keeping up, and may not be able to caption some audio.
- Windows dropped: When the live captioning system is struggling to keep up with the audio in real time, it will drop the incoming unprocessed audio (the "window") and start again. If the system drops multiple windows in a row, some audio may never get captioned. A windows dropped event is worse than a buffer full event.
- Be aware that the CPU is averaged and doesn't give a good idea of how "spiky" live captioning can be.

1 call

| Model | Threads | CPU (avg) | Memory | < 2s | < 4s | < 6s | < 8s | Buffer full | Windows dropped |
|-------|---------|-----------|--------|------|------|------|------|-------------|-----------------|
| Tiny  | 1       | 15.1%     | 1.31GB | 90%  | 100% | 100% | 100% | 3           | 1               |
| Base  | 1       | 14.7%     | 1.41GB | 32%  | 92%  | 99%  | 100% | 9           | 11              |
| Small | 1       | -         | -      | -    | -    | -    | -    | -           | -               |
| Tiny  | 2       | 16.4%     | 1.32GB | 96%  | 100% | 100% | 100% | 1           | 0               |
| Base  | 2       | 21.4%     | 1.42GB | 92%  | 99%  | 100% | 100% | 1           | 1               |
| Small | 2       | 27.1%     | 1.87GB | 40%  | 80%  | 97%  | 100% | 26          | 14              |
| Tiny  | 3       | 16.7%     | 1.33GB | 95%  | 100% | 100% | 100% | 0           | 0               |
| Base  | 3       | 23.1%     | 1.42GB | 95%  | 100% | 100% | 100% | 0           | 0               |
| Small | 3       | 35.5%     | 1.86GB | 33%  | 86%  | 96%  | 98%  | 20          | 12              |
| Tiny  | 4       | 17.2%     | 1.32GB | 98%  | 100% | 100% | 100% | 1           | 0               |
| Base  | 4       | 25.0%     | 1.42GB | 98%  | 100% | 100% | 100% | 0           | 0               |
| Small | 4       | 35.4%     | 1.86GB | 31%  | 86%  | 96%  | 100% | 10          | 7               |


2 simultaneous calls

| Model | Threads | CPU (avg) | Memory | < 2s | < 4s | < 6s | < 8s | Buffer full | Windows dropped |
|-------|---------|-----------|--------|------|------|------|------|-------------|-----------------|
| Tiny  | 1       | 29.7%     | 2.02GB | 87%  | 98%  | 100% | 100% | 7           | 0               |
| Base  | 1       | 29.7%     | 2.25GB | 34%  | 88%  | 95%  | 97%  | 44          | 22              |
| Tiny  | 2       | 33.4%     | 2.05GB | 97%  | 100% | 100% | 100% | 1           | 0               |
| Base  | 2       | 45.8%     | 2.26GB | 89%  | 99%  | 100% | 100% | 0           | 2               |
| Tiny  | 3       | 38.1%     | 2.05GB | 97%  | 100% | 100% | 100% | 0           | 0               |
| Base  | 3       | 54.5%     | 2.26GB | 93%  | 99%  | 100% | 100% | 4           | 1               |
| Tiny  | 4       | 43.7%     | 2.05GB | 96%  | 100% | 100% | 100% | 2           | 0               |
| Base  | 4       | 63.8%     | 2.28GB | 93%  | 100% | 100% | 100% | 3           | 2               |

3 simultaneous calls

| Model | Threads | CPU (avg) | Memory | < 2s | < 4s | < 6s | < 8s | Buffer full | Windows dropped |
|-------|---------|-----------|--------|------|------|------|------|-------------|-----------------|
| Tiny  | 1       | 47.5%     | 2.80GB | 86%  | 98%  | 100% | 100% | 9           | 0               |
| Base  | 1       | 48.3%     | 3.19GB | 32%  | 87%  | 97%  | 99%  | 49          | 35              |
| Tiny  | 2       | 55.9%     | 2.81GB | 94%  | 100% | 100% | 100% | 2           | 1               |
| Base  | 2       | 72.7%     | 3.20GB | 75%  | 97%  | 100% | 100% | 21          | 16              |
| Tiny  | 3       | 65.2%     | 2.81GB | 94%  | 100% | 100% | 100% | 4           | 0               |
| Base  | 3       | 85.2%     | 3.14GB | 68%  | 94%  | 99%  | 100% | 27          | 21              |
| Tiny  | 4       | 84.9%     | 2.85GB | 74%  | 92%  | 97%  | 99%  | 33          | 21              |
| Base  | 4       | 95.9%     | 3.15GB | 66%  | 84%  | 91%  | 98%  | 101         | 47              |


### Description of the methodology

We used a 10 minute call between 4 participants. The call script can be found in the load testing subpackage of the calls repository (TODO: link when merged). The script includes moments where two participants talk over one another to provide a realistic stress-test of the live captioning system. Each participant's lines were sent to AWS Polly text-to-speech and then the audio was "said" on the call. The call's audio was processed and live captions were sent to all call participants, as it would be on a call between real people. At the moment we do not have a measure for quality or accuracy of the live transcriptions.
For the simultaneous calls, the second (and third) calls were started in different channels one minute (and two minutes) after the first call. This prevented the "talking over" portions of the call to line-up on every call and create an unrealistic amount of load.

### Summary

The `Small` model is not recommended for live captioning; currently it is too slow to deliver real time captions. `Base` provides better quality captions than `Tiny`, but `Tiny` is surprisingly capable. However, `Tin`y struggles with accents, muffled voices, and noise.
`Base` requires at least 2 threads for real time captioning, and works best with 3 or 4 threads. `Tiny` is able to provide real-time captioning with 1 thread, but 2 threads are better.
We recommend picking a model and thread count based on the number of simultaneous calls expected:
- 1 call: `Base` with 4 threads
- 2 calls: `Base` with 4 or 3 threads, or `Tiny` with 3 threads
- 3 calls: `Tin`y with 3 or 2 threads
- more than 3 calls: `Tiny` with 2 or 1 threads, and consider horizontally scaling (see [Scalability](#Scalability)). 

Note: The `c7g.2xlarge` performs better than the `c6i.2xlarge`, and will give breathing room for the recommendations above. 

## Scalability

Starting in version `v0.3.2`, this service includes support for horizontal scalability. This can be achieved by adding an HTTP load balancer in front of multiple `calls-offloader` instances, and configuring the [Job Service URL](https://docs.mattermost.com/configure/plugins-configuration-settings.html#job-service-url) setting accordingly to point to the balancer's host.

#### Example (nginx)

This is an example config for load-balancing the service using [`nginx`](https://www.nginx.com/):

```
upstream backend {
	server 10.0.0.1:4545;
	server 10.0.0.2:4545;
}

server {
	listen 4545 default_server;
	listen [::]:4545 default_server;

	location / {

		proxy_set_header Host $http_host;
		proxy_set_header X-Real-IP $remote_addr;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
		proxy_set_header X-Forwarded-Proto $scheme;
		proxy_set_header X-Frame-Options SAMEORIGIN;
		proxy_buffers 256 16k;
		proxy_buffer_size 16k;
		proxy_read_timeout 300s;
		proxy_pass http://backend;
	}
}
```

> **_Note_** 
> If deploying in a Kubernetes environment, scaling is automatically handled by the default `ClusterIP` service type without needing extra configuration.
