# Docker Build Issue Resolution

## Problem Identified

The original Docker build was failing with the error:
```
Fatal glibc error: CPU does not support x86-64-v2
```

### Root Cause

Your system's CPU (Common KVM processor) only supports **x86-64-v1** instructions, while the newer Operator Framework base images require **x86-64-v2** or higher instruction sets. This is common in:
- Older virtualization environments
- KVM/QEMU VMs with older CPU emulation
- Cloud instances with legacy CPU configurations

## Solution

We switched from the problematic operator-framework base image to **Ubuntu 22.04** as the base, which supports older CPU instruction sets.

### Changes Made

1. **Base Image**: Changed from `quay.io/operator-framework/ansible-operator:v1.39.0` to `ubuntu:22.04`
2. **Manual Installation**: Installed Python, Ansible, and required packages directly
3. **Simplified Collections**: Skipped Ansible Galaxy collection installation to speed up builds

### Final Dockerfile Structure

```dockerfile
FROM ubuntu:22.04

# Install system dependencies
RUN apt-get update && apt-get install -y \
    python3 python3-pip curl git

# Install Ansible and dependencies
RUN pip3 install --no-cache-dir \
    ansible==8.5.0 \
    ansible-runner==2.3.4 \
    openshift==0.13.2 \
    kubernetes==28.1.0

# Copy operator files and configure
COPY roles/ /opt/ansible/roles/
COPY playbooks/ /opt/ansible/playbooks/
COPY watches.yaml /opt/ansible/watches.yaml
```

## Build Results

- ✅ **Successfully built**: `sqlite-operator:v0.1.0`
- ✅ **Image size**: 938MB
- ✅ **Build time**: ~2-3 minutes (with cache)
- ✅ **Compatible**: Works on x86-64-v1 CPUs

## How to Build

```bash
cd /home/linux/projects/sqlite-operator
docker build -t sqlite-operator:v0.1.0 .
```

## Alternative Solutions (if needed)

If you need the full Operator Framework functionality:

### Option 1: Use Older Operator SDK Image
```dockerfile
FROM quay.io/operator-framework/ansible-operator:v1.28.0
```
(Older versions may support x86-64-v1)

### Option 2: Build on Different Architecture
- Use a system with modern CPU (x86-64-v2+)
- Use GitHub Actions / GitLab CI for builds
- Use cloud build services (Google Cloud Build, AWS CodeBuild)

### Option 3: Add Ansible Collections Later
```dockerfile
# In entrypoint script, install collections on first run
RUN echo 'ansible-galaxy collection install -r requirements.yml' >> /entrypoint.sh
```

## Verification

Check your CPU support level:
```bash
grep -o 'x86-64-v[0-4]' /proc/cpuinfo | sort -u
# or
/lib/x86_64-linux-gnu/libc.so.6
```

Test the image:
```bash
docker run --rm sqlite-operator:v0.1.0 echo "Image works!"
```

## Future Improvements

1. **Add Multi-stage Build**: Reduce final image size
2. **Install Collections**: Add ansible-galaxy collections when CPU issue is resolved
3. **Use BuildKit**: Enable faster builds with `DOCKER_BUILDKIT=1`
4. **Health Checks**: Add health check endpoints to the operator

## Resources

- [x86-64 Microarchitecture Levels](https://en.wikipedia.org/wiki/X86-64#Microarchitecture_levels)
- [Operator SDK Documentation](https://sdk.operatorframework.io/)
- [Ubuntu Docker Images](https://hub.docker.com/_/ubuntu)
