• Script ist erstellt: scripts/db_backup.sh

Es kann:

- backup (MySQL Dump + .sql.gz)
- restore <file.sql|file.sql.gz>
- list
- latest
- automatische Retention (--retention-days, Default 14)

Cron-Beispiel (DB-Stack auf VPS, täglich 03:15):

15 3 * * * /srv/pixelfox/scripts/db_backup.sh backup \
--compose-file /srv/mysql/docker-compose.yml \
--env-file /srv/mysql/.env \
--backup-dir /srv/backups/pixelfox-db \
--retention-days 14 >> /var/log/pixelfox-db-backup.log 2>&1

Restore-Befehle:

# konkretes Backup einspielen
/srv/pixelfox/scripts/db_backup.sh restore /srv/backups/pixelfox-db/pixelfox_db_20260228_031500.sql.gz \
--compose-file /srv/mysql/docker-compose.yml \
--env-file /srv/mysql/.env

# letztes Backup einspielen
LATEST=$(/srv/pixelfox/scripts/db_backup.sh latest --backup-dir /srv/backups/pixelfox-db)
/srv/pixelfox/scripts/db_backup.sh restore "$LATEST" \
--compose-file /srv/mysql/docker-compose.yml \
--env-file /srv/mysql/.env

Hinweis: Ich habe Syntax/Usage geprüft, aber keinen echten Dump gegen eine laufende DB in dieser Umgebung ausgeführt.