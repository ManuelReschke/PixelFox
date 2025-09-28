# PixelFox Database (MySQL 8.4) – Production

This guide explains how to run the MySQL server for PixelFox on a dedicated VPS using Docker Compose.

## Prerequisites
- Docker and Docker Compose plugin installed
- A separate App VPS that will connect on port 3306
- Firewall to restrict 3306/tcp to the App VPS only

## Deploy
```
sudo mkdir -p /srv/mysql/data
cd /srv/mysql
cp /path/to/repo/docker/prod/db.compose.yml ./docker-compose.yml
cp /path/to/repo/docker/prod/.env.db.example ./.env  # or create minimal .env with DB_* vars
# Edit .env to set strong DB_ROOT_PASSWORD and DB_PASSWORD

docker compose up -d
```

The DB listens on `0.0.0.0:3306` (by Compose). Use the firewall to only allow the App VPS IP.

## Firewall (UFW example)
```
sudo ufw allow ssh
sudo ufw deny 3306/tcp
sudo ufw allow from <APP_VPS_IP> to any port 3306 proto tcp
sudo ufw enable
```

## Verify
From the App VPS or your workstation (if allowed):
```
mysql -h <DB_HOST> -u pixelfox -p pixelfox_db -e 'SELECT 1;'
```

## Backups
Nightly mysqldump example (cron):
```
mysqldump -h 127.0.0.1 -u root -p"$DB_ROOT_PASSWORD" --single-transaction \
  pixelfox_db | gzip > /srv/mysql/backup/pixelfox_db-$(date +%F).sql.gz
```
Store backups on encrypted storage or offsite. Test restore regularly.

## Hardening
- Disable remote root login, keep a separate admin user
- Use strong passwords and rotate periodically
- Keep the MySQL image updated
