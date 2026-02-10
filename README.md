# VPS Deploy System

A reusable webhook-based auto-deployment system for Docker projects. GitHub Actions builds your images, triggers a webhook on the VPS, and a watcher service detects the job and runs your project's deploy script. Other projects integrate by copying a workflow file and a deploy script from the `templates/` directory.

## How It Works

```
git push (main)
      |
      v
GitHub Actions
  1. Build Docker images (matrix)
  2. Push to GHCR
  3. Sign payload (HMAC-SHA256)
  4. POST /webhook/github
      |
      v
Webhook Server (Go/Gin, port 8000)
  - Validates signature
  - Validates ref == refs/heads/main
  - Creates job folder in /data/webhook_jobs/{repo_name}/
      |
      v
Watcher Service (systemd, polls every 60s)
  - Detects job folder
  - Acquires file lock
  - cd /projects/{repo_name}/
  - git fetch + git reset --hard
  - Runs scripts/deploy
      |
      v
Deploy Script (per-project)
  - docker compose pull
  - docker compose up -d
  - docker image prune
```

## Components

| Component | Description |
|-----------|-------------|
| **Webhook Server** | Go/Gin HTTP server that receives GitHub push events, verifies HMAC-SHA256 signatures, and queues deploy jobs by creating folders in a shared Docker volume. |
| **Watcher** | Bash script running as a systemd service. Polls for job folders, acquires a file lock, updates the git repo, and executes the project's `scripts/deploy`. |
| **Nginx** | Reverse proxy with automatic HTTPS. Routes `/webhook/github` to the webhook container. Config is generated from templates via `create_nginx_conf`. |
| **Certbot** | Handles Let's Encrypt certificate creation and automatic renewal (every 12h). |
| **Data Volume** | Alpine container that holds shared data: scripts, nginx config, certificates, and webhook job folders across all services. |

## Quick Start

See [`DEV_GUIDE.md`](DEV_GUIDE.md) for the full step-by-step server setup (SSH hardening, Fail2Ban, UFW, Docker, bootstrap, etc.).

Summary of key steps:

1. Provision a VPS with Docker (e.g., Ubuntu 24.04)
2. Create a non-root user, harden SSH, set up Fail2Ban and UFW
3. Clone this repo to `/dockerlabs` (or your chosen path)
4. Copy `.env.example` to `.env`, fill in your values
5. Run `data/scripts/bootstrap` (first in development, then in production)
6. Create the watcher systemd service
7. Add `DEPLOY_WEBHOOK_URL` and `DEPLOY_WEBHOOK_SECRET` as GitHub repo secrets

## Adding a New Project

Any project can use this deploy system. Follow these steps:

### 1. Set up the project on the VPS

```sh
# Create the project directory
sudo mkdir -p /projects/{repo_name}
sudo chown -R $USER:$USER /projects/{repo_name}
sudo setfacl -R -m d:g:$USER:rwx /projects/{repo_name}
sudo chmod -R 775 /projects/{repo_name}
sudo chmod g+s /projects/{repo_name}
git config --global --add safe.directory /projects/{repo_name}

# Clone your project
cd /projects/{repo_name}
git clone git@github.com:{owner}/{repo}.git .
```

### 2. Create the project's `.env`

```sh
# Use the template as a starting point
cp /path/to/vps-setup/templates/env-example /projects/{repo_name}/.env
# Edit with your values
vim /projects/{repo_name}/.env
```

Required variables (see [`templates/env-example`](templates/env-example)):
- `CURRENT_ENV=production`
- `DEPLOY_BRANCH=main`

### 3. Add the deploy script to your project

Copy [`templates/scripts-deploy`](templates/scripts-deploy) to your project:

```sh
mkdir -p /projects/{repo_name}/scripts
cp /path/to/vps-setup/templates/scripts-deploy /projects/{repo_name}/scripts/deploy
chmod +x /projects/{repo_name}/scripts/deploy
```

Customize the `TODO` markers for your project (migrations, seeds, etc.).

### 4. Add the GitHub Actions workflow

Copy [`templates/github-workflow-deploy.yaml`](templates/github-workflow-deploy.yaml) to your project:

```sh
mkdir -p .github/workflows
cp /path/to/vps-setup/templates/github-workflow-deploy.yaml .github/workflows/deploy.yaml
```

Edit the `TODO` markers: workflow name, matrix services, build-args.

### 5. Add GitHub repository secrets

In your repo: **Settings > Secrets and variables > Actions > New repository secret**

- `DEPLOY_WEBHOOK_URL` - `https://your-domain.com/webhook/github`
- `DEPLOY_WEBHOOK_SECRET` - same value as `GITHUB_WEBHOOK_SECRET` in the VPS `.env`

### 6. (Optional) Add nginx upstream

If your project exposes HTTP endpoints, add an upstream and location block to the nginx template at `data/nginx/templates/app.conf.template`, then recompile:

```sh
docker compose exec data_vol sh -c "/data/scripts/create_nginx_conf app.conf.template"
docker compose up -d nginx --force-recreate
```

## Essential Commands

### Watcher

```sh
# Status
sudo systemctl status webhook-watcher

# Restart
sudo systemctl restart webhook-watcher

# Logs (follow)
sudo journalctl -u webhook-watcher.service -f

# Create the service (first time)
sudo vim /etc/systemd/system/webhook-watcher.service
sudo systemctl daemon-reload
sudo systemctl enable webhook-watcher
sudo systemctl start webhook-watcher

# Remove the service
sudo systemctl stop webhook-watcher
sudo systemctl disable webhook-watcher
sudo rm /etc/systemd/system/webhook-watcher.service
sudo systemctl daemon-reload
```

### Fail2Ban

```sh
# General status
sudo fail2ban-client status

# SSH jail status (banned IPs, counters)
sudo fail2ban-client status sshd

# Unban an IP
sudo fail2ban-client set sshd unbanip 192.168.1.50

# Unban all
sudo fail2ban-client unban --all

# Ban an IP manually
sudo fail2ban-client set sshd banip 203.0.113.45

# Logs (follow)
sudo journalctl -f -u fail2ban

# SSH login attempts
sudo journalctl -f -u ssh
```

### Nginx / Bootstrap

```sh
# Recompile nginx config from template
docker compose exec data_vol sh -c "/data/scripts/create_nginx_conf app.conf.template"
docker compose up -d nginx --force-recreate

# Full bootstrap (development first, then production)
/dockerlabs/data/scripts/bootstrap
```

### Docker

```sh
# Running containers
docker compose ps

# Logs (follow)
docker compose logs -f [service]

# Exec into a container
docker compose exec [service] sh
```

### UFW

```sh
# Firewall status
sudo ufw status verbose
```

## Environment Variables

Variables from `.env.example` for the VPS setup itself:

| Variable | Description | Example |
|----------|-------------|---------|
| `CURRENT_ENV` | Environment mode | `production` or `development` |
| `DOMAINS` | Space-separated domain list | `"example.com www.example.com"` |
| `EMAIL` | Email for Let's Encrypt notifications | `admin@example.com` |
| `GITHUB_WEBHOOK_SECRET` | Shared secret for HMAC signature verification | `(generate with python3 -c "import secrets; print(secrets.token_hex(32))")` |
| `DEPLOY_BRANCH` | Branch the watcher pulls | `main` |
| `APP_UID` | Application user ID | `1011` |
| `APP_GID` | Application group ID | `1011` |

## Project Structure

```
.
├── .github/workflows/
│   └── deploy.yaml              # CI/CD: build images + trigger webhook
├── data/
│   ├── nginx/
│   │   ├── templates/           # Nginx config templates (envsubst)
│   │   ├── nginx.conf           # Main nginx config
│   │   └── letsencrypt/conf/    # SSL params
│   └── scripts/
│       ├── bootstrap            # First-time setup (dev + prod)
│       ├── watcher              # Polls for deploy jobs (systemd)
│       ├── create_nginx_conf    # Compiles nginx templates
│       ├── backup_volumes       # Docker volume backups
│       ├── certbot_create_prod_certs
│       ├── certbot_renewal      # Runs inside certbot container
│       └── nginx_renewal        # Runs inside nginx container
├── scripts/
│   └── deploy                   # This repo's own deploy script
├── templates/
│   ├── github-workflow-deploy.yaml  # Workflow template for other projects
│   ├── scripts-deploy               # Deploy script template for other projects
│   └── env-example                  # .env template for other projects
├── webhook/
│   └── main.go                  # Webhook server (Go/Gin)
├── compose.yaml                 # Docker Compose services
├── Dockerfile                   # Webhook server (multi-stage)
├── Dockerfile.nginx             # Nginx + renewal script
├── Dockerfile.certbot           # Certbot + renewal script
├── Dockerfile.data_vol          # Shared data volume
├── .env.example                 # Environment variables template
└── DEV_GUIDE.md                 # Full server setup guide
```
