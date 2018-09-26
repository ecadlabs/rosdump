Rosdump
=========

[![CircleCI](https://circleci.com/gh/ecadlabs/rosdump/tree/master.svg?style=svg)](https://circleci.com/gh/ecadlabs/rosdump/tree/master)


rosdump is a tool for backing up and optionally tracking configuration of
RouterOS devices.

Use rosdump to:

- Backup Mikrotik network device configurations to your local file system
- Backup Mikrotik network device configurations to a git repository
- Run as a daemon that backs up devices on a predefined schedule


# Quick Start

The following example uses the docker image to backup a RouterOS device. Create
a config named `rosdump.yml` on your computer using the example below. 

Specify the IP address, admin user and password of your own device.

## Config example that writes backup to local filesystem

```yaml
version: 1

devices:
  list:
    - host: 192.168.88.1
  common: # Overrides per-device settings
    driver: ssh-command
    timeout: 30s # See https://golang.org/pkg/time/#ParseDuration
    command: export
    username: admin
    password: password

storage:
  driver: file
  timeout: 30s
  path: '/opt/backups/{{.host}}/{{.time.UTC.Format "2006-01-02T15:04:05Z07:00"}}'

timeout: 30s # Optional timeout for a whole work cycle
interval: 4h # Duration between backups when running as a daemon
```

Assuming your config file is named `rosdump.yaml`, and you have docker
installed on your computer, run the following command:

```
docker run --rm \
        -v $(realpath rosdump.yml):/etc/rosdump.yml \
        -v $(realpath backups):/opt/backups ecadlabs/rosdump
```

You will now have a backup of your configuration in a directory named
`backups/` in your present working directory.

# Configuration Schema

## Exporter drivers

### Common options

| Name    | Type                | Default     | Required | Description |
| ------- | ------------------- | ----------- | -------- | ----------- |
| driver  | string              | ssh-command |          | Driver name |
| timeout | string/duration[^1] |             |          |             |

### ssh-command

| Name          | Type    | Default | Required | Description                       |
| ------------- | ------- | ------- | -------- | --------------------------------- |
| name          | string  |         |          | Optional device name              |
| host          | string  |         | ✓        | Host address                      |
| port          | integer | 22      |          | SSH port                          |
| username      | string  |         | ✓        | User name                         |
| password      | string  |         |          | Password                          |
| identity_file | string  |         |          | SSH private key file              |
| command       | string  | export  |          | Command to run on a remote device |

## Storage drivers

### Common options

| Name    | Type                | Default | Required | Description |
| ------- | ------------------- | ------- | -------- | ----------- |
| driver  | string              |         | ✓        | Driver name |
| timeout | string/duration[^1] |         |          |             |

### file

| Name     | Type            | Default | Required | Description          |
| -------- | --------------- | ------- | -------- | -------------------- |
| path     | string/template |         | ✓        | Destination path     |
| compress | boolean         | false   |          | Use gzip compression |

### git

| Name             | Type            | Default | Required | Description                                                  |
| ---------------- | --------------- | ------- | -------- | ------------------------------------------------------------ |
| repository_path  | string          |         |          | Local repository path. In-memory storage will be used if not specified. |
| url              | string          |         |          | URL to clone if the specified repository is not initialised. In-memory storage is always empty at startup so cloning will be performed anyway in this case. |
| pull             | boolean         |         |          | Pull changes from remote repository on startup (only if cloning is not required, see above). |
| username         | string          |         |          | User name (overrides one from URL)                           |
| password         | string          |         |          | Password (overrides one from URL)                            |
| identity_file    | string          |         |          | SSH private key file                                         |
| remote_name      | string          |         |          | Name of the remote to be pulled. If empty, uses the default. |
| reference_name   | string          |         |          | Remote branch to clone. If empty, uses HEAD.                 |
| push             | boolean         |         |          | Push after commit                                            |
| ref_specs        | array           |         |          | Specifies what destination ref to update with what source    |
| destination_path | string/template |         | ✓        | Target path template relative to work tree                   |
| name             | string          |         | ✓        | Author name                                                  |
| email            | string          |         | ✓        | Author email                                                 |
| commit_message   | string/template |         | ✓        | Commit message                                               |

[^1]: https://golang.org/pkg/time/#ParseDuration

#### Example config for git storage

```yaml
version: 1

devices:
  list:
    - host: 192.168.88.1
  common:
    driver: ssh-command
    timeout: 30s
    command: export
    username: admin
    identity_file: /etc/rosdump/routeros_admin_private_key

storage:
  driver: git
  timeout: 30s
  url: git@github.com:yourorg/networkbackups.git
  identity_file: /etc/rosdump/git_deploy_key
  destination_path: '{{.host}}'
  push: true
  name: Network Backup
  email: networkbackup@example.net
  commit_message: 'Rosdump backup {{.time.UTC.Format "2006-01-02T15:04:05Z07:00"}}'

timeout: 30s
interval: 4h
```

## Template data fields (transaction metadata)

Currently `ssh-command` driver exposes all its options (except password) as a transaction metadata. Additionally `time` field is set to transaction timestamp (see the description of Go `time.Time` type).

