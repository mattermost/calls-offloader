# Deploying and running the `calls-offloader` service

## Deployment (sample diagram)

![diagram](assets/diagram.png)

## Prerequisites

- An [Enterprise](https://docs.mattermost.com/about/editions-and-offerings.html#mattermost-enterprise) licensed Mattermost installation (>= v7.6) with [Calls](https://github.com/mattermost/mattermost-plugin-calls) installed.
- An instance with a working version of [Docker](https://www.docker.com/) installed.

## Installation

> **_Note:_** The following steps are targeting Ubuntu based systems but they should be easily adaptable to other Linux distributions with none to mininimal changes.

As first step you should download the latest official `calls-offloader` version which can be found at https://github.com/mattermost/calls-offloader/releases:

```
wget https://github.com/mattermost/calls-offloader/releases/download/vx.x.x/calls-offloader-linux-amd64
```

Move the binary to the local installation path:

```
sudo mv calls-offloader-linux-amd64 /usr/local/bin/calls-offloader
```

Create a new system user to own and run the service:

```
sudo useradd --system --user-group mattermost
```

It's important that the user running the `calls-offloader` binary is part of the *docker* group as the service requires access to the docker API to run jobs:

```
sudo usermod -a -G docker mattermost
```

Give ownership to the service binary:

```
sudo chown mattermost:mattermost /usr/local/bin/calls-offloader
```

Give executable permissions to the service binary:

```
sudo chmod +x /usr/local/bin/calls-offloader
```

## Running

To start the service you can create and enable the following *systemd* file:

```
sudo touch /lib/systemd/system/calls-offloader.service
```

```
[Unit]
Description=calls-offloader
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/calls-offloader
Restart=always
RestartSec=10
User=mattermost
Group=mattermost
Environment=API_SECURITY_ALLOWSELFREGISTRATION=true

[Install]
WantedBy=multi-user.target
```

> **_Note:_** By default the service starts even if no configuration file is provided. In such case default values are used. In the service file above we are overriding a config setting through environment variables:
>    - `api.security.allow_self_registration` We set this to `true` so that clients (Mattermost instances) can automatically self register and authenticate to the service without manually having to create accounts. This is fine as long as the service is running in an internal/private network.

Load the service file:

```
sudo systemctl daemon-reload
```

Enable and start the service:

```
sudo systemctl enable --now /lib/systemd/system/calls-offloader.service
```

Verify that the service is running:

```
curl http://localhost:4545/version
```

## Configuration

Configuration for the service is fully documented in-place through the [`config.sample.toml`](../config/config.sample.toml) file.

## Running with Mattermost Calls

The last step is to configure the calls side to use the service. This is done via the **System Console > Plugins > Calls > Job service URL** setting, which in this example will be set to `http://localhost:4545`.

> **_Note:_**
> 1. The client will self-register the first time it connects to the service and store the authentication key in the database. If no client ID is explicitly provided, the diagnostic ID of the Mattermost installation will be used.
> 2. The service URL supports credentials in the form `http://clientID:authKey@hostname`. Alternatively these can be passed through environment overrides to the Mattermost server, namely `MM_CALLS_JOB_SERVICE_CLIENT_ID` and `MM_CALLS_JOB_SERVICE_AUTH_KEY`.
