## Performance and scalability considerations

### Quality profiles

Recording quality can be configured through the [Calls plugin](https://docs.mattermost.com/configure/plugins-configuration-settings.html#call-recording-quality). At this time we provide the following quality profiles:

| Profile | Resolution | Framerate | Bitrate                          |
|--------:|-----------:|----------:|---------------------------------:|
| Low     | 720p       | 15fps     | 1Mbps (video) / 64Kbps (audio)   |
| Medium  | 720p       | 20fps     | 1.5Mbps (video) / 64Kbps (audio) |
| High    | 1080p      | 20fps     | 2.5Mbps (video) / 64Kbps (audio) |

### Benchmarks

These are the results of a series of benchmarks that was conducted to verify the scalability capabilities of the service. All tests were executed on a AWS EC2 `c6i.2xlarge` instance which is the recommended instance class and size (`8vCPU / 16GB RAM`) for Calls recordings:

| Profile | Concurrency | CPU (avg) | Memory (avg) | Recording size (avg) |
|--------:|------------:|----------:|-------------:|---------------------:|
| Low     | 8           | 66%       | 4GB          | 0.5GB/hour           |
| Medium  | 6           | 66%       | 4GB          | 0.7GB/hour           |
| High    | 4           | 72%       | 4GB          | 1.2GB/hour           |

We recommend setting the [`max_concurrent_jobs`](https://github.com/mattermost/calls-offloader/blob/85717457b3e699fd507e8bed4586e82daa19a045/config/config.sample.toml#L33) config option to the values above, based on the quality profile used.

On the Mattermost side it may also be necessary to tune the [`FileSettings.MaxFileSize`](https://docs.mattermost.com/configure/environment-configuration-settings.html#maximum-file-size) setting depending on the profile choosen and the configured [`MaxCallDuration`](https://docs.mattermost.com/configure/plugins-configuration-settings.html#maximum-call-recording-duration).

> **_Note_** 
> If a load-balancer or proxy is in front of Mattermost, extra configuration may be necessary. 
> As an example, `nginx` would likely require `client_max_body_size` to be set accordingly.

> **_Note_** 
> At this time we don't provide an official solution for horizontal scalability of this service.
