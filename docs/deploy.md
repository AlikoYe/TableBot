# Deploying TableBot

This runbook puts TableBot on **one Ubuntu 24.04 VM** (Azure) with HTTPS, then
sets up automatic deploys from GitHub. Follow the steps in order. Each step says
what success looks like. Commands are meant to be copy-pasted.

What runs on the server (all in Docker, started by `deploy/deploy.sh`):

| Service | What it does |
|---|---|
| `postgres` | The database. Not reachable from outside the VM. |
| `migrate` | Applies database migrations, then exits. |
| `api` | Go backend + Telegram bot (long polling) + background worker. |
| `caddy` | Serves the Vue app, proxies `/api`, gets the HTTPS certificate automatically. |

Throughout this guide, replace:

- `<host>` with your VM's DNS name, e.g. `tablebot-g30.westeurope.cloudapp.azure.com`
- `<your-telegram-id>` with your numeric Telegram id (send any message to
  [@userinfobot](https://t.me/userinfobot) to get it)

## Before you start

- **The VM exists**: Ubuntu 24.04, user `azureuser`, you can SSH in with your key.
- **Azure networking**: in the VM's *Networking* page, inbound rules allow
  **22, 80 and 443** (TCP; also 443 UDP if you can). Let's Encrypt needs 80 and 443.
- **DNS name**: in the VM's public IP resource → *Configuration* → *DNS name label*,
  set a label (e.g. `tablebot-g30`). That gives you `<host>`.
- **A production bot**: create a *second* bot in [@BotFather](https://t.me/BotFather)
  for production and keep its token handy. Your laptop keeps using the dev bot (see
  "One token, one place" below).

> ### One token, one place
> Telegram lets **only one program at a time** read a bot's updates (long polling).
> If the same `BOT_TOKEN` runs on your laptop and on the server, they steal updates
> from each other and the bot seems to ignore buttons and `/start` at random.
> **Laptop = dev bot token, server = prod bot token. Never mix them.**

## 1. SSH into the server

On your Mac:

```bash
ssh azureuser@<host>
```

✅ You see a prompt like `azureuser@tablebot-vm:~$`.

## 2. Run the server setup, then log out and back in

On the server:

```bash
curl -fsSL https://raw.githubusercontent.com/AlikoYe/TableBot/main/deploy/server-setup.sh -o server-setup.sh
sudo bash server-setup.sh
```

This takes a few minutes. It installs Docker, adds a 2 GB swapfile, turns on the
firewall (SSH, 80, 443), enables automatic security updates, clones the app into
`/opt/tablebot`, and installs a nightly database backup (03:00 server time, which is UTC on
Azure). It is safe to run again if something fails halfway.

✅ It ends with `==> Server setup finished.` and a list of next steps.

Now log out and back in, so your user can run Docker:

```bash
exit
```

```bash
ssh azureuser@<host>
```

```bash
docker ps
```

✅ `docker ps` prints an empty table (`CONTAINER ID   IMAGE ...`) with no "permission denied".

## 3. Create the production config

On the server:

```bash
cd /opt/tablebot
cp deploy/.env.production.example .env
openssl rand -hex 24
```

Copy the random string that `openssl` printed, then edit the file:

```bash
nano .env
```

Set:

- `POSTGRES_PASSWORD` = the random string
- `BOT_TOKEN` = the **production** bot token
- `WEBAPP_URL` = `https://<host>`
- `SITE_ADDRESS` = `<host>` (no `https://`, no trailing slash)

Save with `Ctrl+O`, `Enter`, exit with `Ctrl+X`. Then lock the file down:

```bash
chmod 600 .env
```

✅ `grep -c CHANGE_ME .env` prints `0`.

## 4. First deploy

```bash
bash /opt/tablebot/deploy/deploy.sh
```

The first run builds the images (3–6 minutes) and Caddy requests the HTTPS certificate.

✅ It ends with:

```
==> Deploy OK: https://<host>/api/health -> {"status":"ok"}
```

followed by a table where `postgres`, `api` and `caddy` are `Up` (`migrate` has exited, which
is expected). Open `https://<host>` in your browser: the app loads with a valid padlock.

If it fails, the script prints the container status and the last log lines of `migrate`,
`api` and `caddy`. Check the troubleshooting table at the end of this page.

## 5. Load demo venues owned by you

```bash
cd /opt/tablebot
make prod-seed OWNER=<your-telegram-id>
```

✅ The output ends with `COMMIT`. You now own the 3 demo venues (restaurant, billiards, cafe).

## 6. Try it in Telegram

1. Open your production bot in Telegram and send `/start`.
2. ✅ The bot answers with a welcome message and an **📱 Open TableBot** button.
3. ✅ The menu button next to the message field says **Open TableBot** (the server sets it
   from `WEBAPP_URL` on startup; restart Telegram if you still see the old one).
4. Open the app, book a table. ✅ You (as the owner) get a message with **✅ Confirm** and
   **❌ Reject** buttons.

## 7. Automatic deploys from GitHub

GitHub Actions deploys every push to `main` by SSH-ing into the server and running
`deploy.sh`. It uses a **separate key** that is only allowed to run the deploy script.

**7a. Create the deploy key on your Mac:**

```bash
ssh-keygen -t ed25519 -f ~/.ssh/tablebot_deploy -N "" -C "github-actions-deploy"
```

**7b. Allow it on the server, restricted to the deploy script** (run on your Mac):

```bash
echo "restrict,command=\"bash /opt/tablebot/deploy/deploy.sh\" $(cat ~/.ssh/tablebot_deploy.pub)" | ssh azureuser@<host> 'cat >> ~/.ssh/authorized_keys'
```

✅ Test it. This should run a full deploy and end with `Deploy OK`:

```bash
ssh -i ~/.ssh/tablebot_deploy azureuser@<host>
```

**7c. Get the server's host key** (on your Mac):

```bash
ssh-keyscan -t ed25519 <host>
```

✅ One line starting with `<host> ssh-ed25519 AAAA...`. Copy the whole line.

**7d. Add 4 secrets in GitHub**: open the repository → **Settings** → **Secrets and
variables** → **Actions** → **New repository secret**, and add:

| Name | Value |
|---|---|
| `DEPLOY_HOST` | `<host>` |
| `DEPLOY_USER` | `azureuser` |
| `DEPLOY_SSH_KEY` | the **private** key: run `pbcopy < ~/.ssh/tablebot_deploy` on your Mac and paste (starts with `-----BEGIN OPENSSH PRIVATE KEY-----`) |
| `DEPLOY_KNOWN_HOSTS` | the line from step 7c |

✅ Four secrets listed. Until `DEPLOY_HOST` exists, the workflow skips the deploy with a notice.

## 8. Merge something and watch it deploy

Merge any pull request into `main` (or open **Actions** → **deploy** → **Run workflow**).
In the **Actions** tab, the `deploy` workflow runs `check` (Go vet + tests, frontend build)
and then `deploy`.

✅ Both jobs are green, and the `deploy` log ends with `Deploy OK: https://<host>/api/health`.
A failed deploy shows the container logs right in the job output.

## Everyday commands (on the server, in `/opt/tablebot`)

| Task | Command |
|---|---|
| See container status | `make prod-ps` |
| Follow all logs | `make prod-logs` (Ctrl+C to stop) |
| Logs of one service | `docker compose -p tablebot --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml logs --tail 200 api` |
| Redeploy what is checked out | `make prod-up` |
| Deploy latest `main` | `bash deploy/deploy.sh` |
| Restart one service | `docker compose -p tablebot --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml restart api` |
| Database shell | `make prod-psql` |
| Backup now | `make prod-backup` (files in `/opt/tablebot/backups/`, newest 7 kept) |

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Deploy fails, caddy log shows `obtaining certificate` errors / `timeout during connect` | Let's Encrypt cannot reach the VM on 80/443 | Azure *Networking*: inbound **80 and 443** allowed. On the VM: `sudo ufw status` lists 80/tcp and 443/tcp. Wait a minute and run `make prod-up` again. |
| caddy log: `no such host` / `NXDOMAIN` for your domain | DNS name label missing or `SITE_ADDRESS` has a typo | Check the public IP's *DNS name label*; `SITE_ADDRESS` must be exactly `<host>` (no `https://`, no `/`). `nslookup <host>` from your Mac must show the VM's IP. |
| caddy log: `too many certificates` / rate limit | Many failed attempts in a short time | Fix the cause above, then wait (Let's Encrypt limits reset within an hour to a week). Don't loop deploys. |
| Browser shows **502 Bad Gateway** | `api` is not running | `make prod-ps`; `... logs --tail 200 api migrate`. Common: wrong `POSTGRES_PASSWORD` after the DB was created (see below), or `BOT_TOKEN` empty (api refuses to start outside dev). |
| api log: `password authentication failed` | `POSTGRES_PASSWORD` changed after the first start | The database keeps the password from its first start. Put the old one back into `.env`. Only if the DB holds nothing you need: `docker compose ... down -v` deletes all data, then deploy again. |
| Bot does not answer `/start` | Wrong token, or the same token runs somewhere else | Check `BOT_TOKEN` is the **prod** bot. Make sure no laptop or other server runs with it. api log line `telegram bot error ... Conflict: terminated by other getUpdates request` confirms a second consumer. Then `make prod-up`. |
| Menu button does not open the app | `WEBAPP_URL` wrong | Must be `https://<host>`. `make prod-up`, then restart the Telegram app. |
| Owner gets no booking messages | Owner never pressed **Start** in the bot, or wrong `OWNER` id | Send `/start` to the bot; re-run `make prod-seed OWNER=<your-telegram-id>`. api log `telegram send failed ... chat not found` means the id is wrong or the user never started the bot. |
| Deploy says `cannot talk to Docker` | You did not log out/in after setup | `exit`, SSH in again. |
| `No space left on device` | Old images / logs / backups | `df -h /`; `docker system df`; `docker image prune -af` (keeps running containers); delete old files in `/opt/tablebot/backups`. Container logs are capped at 30 MB per service. |
| GitHub deploy: `Host key verification failed` | `DEPLOY_KNOWN_HOSTS` wrong or the VM was recreated | Redo step 7c and update the secret. |
| GitHub deploy: `Permission denied (publickey)` | `DEPLOY_SSH_KEY` or `authorized_keys` line wrong | Redo 7b and 7d; test with `ssh -i ~/.ssh/tablebot_deploy azureuser@<host>`. |

### Roll back to the previous version

On the server:

```bash
cd /opt/tablebot
git log --oneline -5
git reset --hard HEAD~1
make prod-up
```

`make prod-up` deploys the checked-out commit without pulling. The next push to `main`
(or `bash deploy/deploy.sh`) moves you forward again, so also revert the bad commit on GitHub.
Database migrations are not rolled back automatically.

### Restore a backup

This **replaces** the current database with the backup.

```bash
cd /opt/tablebot
ls -lh backups/
C="docker compose -p tablebot --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml"
$C stop api
gunzip -c backups/tablebot-YYYYMMDD-HHMMSS.sql.gz | $C exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
$C start api
```

✅ `psql` prints `DROP ...`, `CREATE ...`, `COPY n` lines without `ERROR`, and the app shows the
restored data.
