##  System Information

### Linux distribution

``` openSUSE 42.2/ Centos7/ Ubuntu.. ```

### Terraform version

```sh
terraform -v
```

### Provider versions

```sh
terraform-provider-opnsense -version
```

If that gives you "was not built correctly", get the Git commit hash from your local provider repository:

```sh
git describe --always --abbrev=40 --dirty
```
___

## Description of Issue/Question

### Setup

(Please provide the full _main.tf_ file for reproducing the issue (Be sure to remove sensitive information)

### Steps to Reproduce Issue

(Include debug logs if possible and relevant).

___
## Additional information:

Do you have SELinux or Apparmor/Firewall enabled? Some special configuration?
Have you tried to reproduce the issue without them enabled?
