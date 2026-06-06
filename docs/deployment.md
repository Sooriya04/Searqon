# Deployment

---

## Option 1: Run Directly (Local)

```bash
cd go_scraper
go run .
# Listening on :4001
```

---

## Option 2: Compiled Binary

```bash
make build        # produces ./searqon
./searqon         # run it

# Or as a background process
nohup ./searqon > searqon.log 2>&1 &
```

---

## Option 3: Docker

### Build the image
```bash
docker build -t searqon .
```

### Run
```bash
docker run -d \
  --name searqon \
  -p 4001:4001 \
  --restart unless-stopped \
  searqon
```

---

## Option 4: Docker Compose

Searqon alone:
```bash
docker compose up searqon
```

With SearXNG (optional, separate service):
```bash
# SearXNG is behind a profile — opt-in only
docker compose --profile searxng up
```

> Both services are completely independent. Stopping SearXNG does not affect Searqon.

---

## Option 5: systemd Service (Linux)

Create `/etc/systemd/system/searqon.service`:

```ini
[Unit]
Description=Searqon Web Intelligence Engine
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/searqon
ExecStart=/opt/searqon/searqon
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable searqon
sudo systemctl start searqon
sudo systemctl status searqon
```

---

## Nginx Reverse Proxy (Optional)

```nginx
server {
    listen 80;
    server_name searqon.yourdomain.com;

    location / {
        proxy_pass http://127.0.0.1:4001;
        proxy_set_header Host $host;
        proxy_read_timeout 120s;
    }
}
```
