# Docker Notes

This directory is reserved for compose overrides, production manifests, and container runtime helpers.

Current baseline:
- Root `docker-compose.yml` provides local parity for backend/frontend.
- Service-specific image definitions live in `backend/Dockerfile` and `frontend/Dockerfile`.
