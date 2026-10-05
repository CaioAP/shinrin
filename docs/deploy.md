# Deploying Shinrin

Shinrin runs on one free Oracle Cloud server, behind a Cloudflare Tunnel on a
subdomain of your own domain (for example `shinrin.example.com`). Everything
runs in Docker Compose, configured by `deploy/compose.yml`:

| Service | What it does |
|---|---|
| `postgres` | The database (a Docker volume on the server's disk) |
| `migrate` | Applies database migrations, then exits; `api` and `worker` wait for it |
| `api` | The Go API, reachable only inside Docker |
| `worker` | Scheduled routines (prices, filings, news, scoring) |
| `web` | The Nuxt app, the only thing the tunnel exposes |
| `tunnel` | `cloudflared`: connects out to Cloudflare, so the server opens no web ports |
| `backup` | `pg_dump` every night at 03:00 BRT into `deploy/backups/`, kept 14 days |

HTTPS, caching and DDoS protection come from Cloudflare. The server only needs
SSH open.

Cost: nothing, as long as the server stays inside Oracle's Always Free limits
and Cloudflare Tunnel stays free (it is part of Cloudflare's free Zero Trust
plan).

## 1. Create the Oracle server (you)

1. Sign up at [oracle.com/cloud/free](https://www.oracle.com/cloud/free/). Pick
   **São Paulo** (or the region nearest you) as the home region: it cannot be
   changed later, and Always Free servers only run in the home region. Oracle
   asks for a card to verify identity; Always Free resources are not charged.
2. Consider upgrading the account to **Pay As You Go** (Billing → Upgrade).
   Always Free resources stay free on it, and Oracle does not reclaim "idle"
   Always Free servers on paid accounts. Shinrin is quiet most of the day, so
   on a free-only account it can look idle and be stopped. Set a budget alert
   at $1 if you upgrade.
3. Compute → Instances → **Create instance**:
   - Image: **Canonical Ubuntu 24.04** (the aarch64 build is picked
     automatically for Ampere).
   - Shape: **VM.Standard.A1.Flex** (Ampere, arm64), **2 OCPUs, 12 GB** memory.
     Oracle lowered the free Ampere allowance to 2 OCPUs and 12 GB in 2026;
     this fits either way, and Shinrin needs far less.
   - Boot volume: 100 GB (Always Free includes 200 GB of block storage in
     total; check that the console shows the "Always Free eligible" label).
   - Networking: first create the network in another tab (Networking →
     Virtual cloud networks → Start VCN Wizard → **Create VCN with Internet
     Connectivity**, default settings). Then pick **Select existing virtual
     cloud network**, that VCN and its **public subnet**, and switch on
     **Automatically assign public IPv4 address**. (With "Create new virtual
     cloud network" in the instance form, the console greys out the public IP
     toggle.) Without a public IP you can't SSH in and the server can't reach
     the internet, so no Docker install and no Cloudflare Tunnel.
   - Oracle Cloud Agent plugins: the defaults are fine; Shinrin doesn't need
     Cloud Guard Workload Protection.
   - SSH keys: upload your public key (`~/.ssh/id_ed25519.pub`).
   - If creation fails with "Out of capacity", try another availability
     domain or try again later; Ampere capacity comes and goes.
4. Leave the security list as it is (only SSH, port 22, is open). Shinrin
   needs no inbound web ports.

## 2. Prepare the server

```bash
ssh ubuntu@<server-ip>

# Updates (Ubuntu's cloud image already installs security patches
# automatically with unattended-upgrades)
sudo apt update && sudo apt -y upgrade && sudo apt -y install git

# Docker Engine and the Compose plugin (official repository)
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker ubuntu && exit   # log in again so the group applies
```

## 3. Create the Cloudflare Tunnel (you)

1. Cloudflare dashboard → **Zero Trust** → Networks → **Tunnels** → Create a
   tunnel → **Cloudflared** → name it `shinrin`.
2. On the install step, copy only the token from the
   `cloudflared ... --token <TOKEN>` command. You don't need to install
   anything; the `tunnel` container uses the token.
3. **Public hostname**: subdomain `shinrin`, your domain, service type
   **HTTP**, URL **`web:3000`**. Cloudflare creates the DNS record for you.

## 4. Configure and start Shinrin

```bash
ssh ubuntu@<server-ip>
git clone https://github.com/CaioAP/shinrin.git   # a private repo needs a deploy key or token
cd shinrin/deploy
cp .env.example .env
cp shinrin.env.example shinrin.env
nano .env           # POSTGRES_PASSWORD, CLOUDFLARE_TUNNEL_TOKEN
nano shinrin.env    # SHINRIN_MASTER_KEY and your data provider keys
chmod 600 .env shinrin.env
```

- `POSTGRES_PASSWORD`: `openssl rand -base64 24`.
- `SHINRIN_MASTER_KEY`: `openssl rand -base64 32`, generated **once**. Save a
  copy in your password manager: it encrypts users' saved LLM keys, and if it
  is lost every user has to enter their key again. Without it, AI reports are
  switched off in the app.
- The data provider keys are the same `SHINRIN_*` values described in
  `backend/.env.example`.

Then build and start (the first build takes a few minutes on the server):

```bash
docker compose up -d --build
docker compose ps          # migrate "exited (0)", everything else "running"
docker compose logs -f api worker tunnel
```

Open `https://shinrin.<your-domain>`.

## 5. First data load

The worker keeps data fresh on its schedule, but the first load is run once,
in this order (the same as the README):

```bash
docker compose run --rm worker run ibov_members sp500_members
docker compose run --rm worker run b3_prices_eod b3_corporate_actions cvm_fundamentals
docker compose run --rm worker run us_prices_eod sec_fundamentals
docker compose run --rm worker run indicators
docker compose run --rm worker run macro_br tesouro_bonds cvm_news
docker compose run --rm worker run quotes_us news_us quotes_b3 macro_us
docker compose run --rm worker run scoring
```

US prices follow Tiingo's free quota (about 50 tickers an hour), so the first
S&P 500 backfill takes most of a day; the worker continues it on schedule.

## Updating

```bash
cd ~/shinrin && git pull
cd deploy && docker compose up -d --build
```

Migrations run automatically before the new `api` and `worker` start.

## Backups

Nightly dumps land in `~/shinrin/deploy/backups/` on the server. A copy on the
same disk does not survive losing the server, so copy them off it, for example
to your computer:

```bash
rsync -av ubuntu@<server-ip>:shinrin/deploy/backups/ ./shinrin-backups/
```

(Oracle's Always Free Object Storage also works, with `rclone`.)

Restore a dump into a running stack:

```bash
docker compose exec -T backup pg_restore --clean --if-exists -d shinrin < backups/shinrin-YYYYMMDD.dump
```

## Troubleshooting

- **The site shows a Cloudflare 502 or 1033**: `docker compose logs tunnel`
  (wrong token?) and `docker compose ps web`.
- **Sign-in fails with "cross-site request refused"**: the tunnel's public
  hostname must point at `web:3000` over HTTP; Cloudflare must pass the
  original host name through (it does by default).
- **AI reports say "not enabled on this server"**: `SHINRIN_MASTER_KEY` is
  missing from `shinrin.env`; add it and `docker compose up -d`.
- **Disk usage**: `docker system df`; old images go with
  `docker image prune`.
