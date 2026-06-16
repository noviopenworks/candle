module git.acme.local/apps/svc

go 1.26

require git.acme.local/platform/auth v1.2.0

replace git.acme.local/platform/auth => ../auth
