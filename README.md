# Rosdump (Beta)

rosdump is a tool for backing up and tracking configuration of RouterOS devices

Use rosdump to:

- Backup Mikrotik network device configuration to local files
- Backup Mikrotik network device configuration and track changes over time
    using git
- Run as a daemon that backs up devices on a predefined schedule

## Config example

```yaml
version: 1

devices:
  list:
    - options: # Driver-dependent
        host: 192.168.88.1
  common: # Overrides per-device settings
    driver: ssh-command
    timeout: 30s # See https://golang.org/pkg/time/#ParseDuration
    options: # Driver-dependent
      command: export
      username: admin
      password: password

storage:
  driver: file
  timeout: 30s
  options: # Driver-dependent
    path: 'storage/{{.host}}/{{.time.UTC.Format "2006-01-02T15:04:05Z07:00"}}'

timeout: 30s # Optional timeout for a whole work cycle
interval: 4h
```

## Exporter drivers

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

#### Example

```yaml
version: 1

devices:
  list:
    - options:
        host: 192.168.88.1
  common:
    driver: ssh-command
    timeout: 30s
    options:
      command: export
      username: admin
      identity_file: /path/to/routeros_admin_private_key

storage:
  driver: git
  timeout: 30s
  options:
    url: git@github.com:yourorg/networkbackups.git
    identity_file: /path/to/private_git_deploy_key
    destination_path: '{{.host}}'
    push: true
    name: Network Backup
    email: networkbackup@example.net
    commit_message: '{{.time.UTC.Format "2006-01-02T15:04:05Z07:00"}}'

timeout: 30s
interval: 4h
```



## Template data fields (transaction metadata)

Currently `ssh-command` driver exposes all its options (except password) as transaction metadata. Additionally `time` field is set to transaction timestamp (see the descroption of Go `time.Time` type).
