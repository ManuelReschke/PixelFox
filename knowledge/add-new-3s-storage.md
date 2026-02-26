## Beispiel:

Admin -> Speicher -> Speicherpool erstellen

1. Name: B2 Cold Dev
2. Speichertyp: S3 Compatible
3. Performance-Tier: cold
4. Basis-Pfad: s3://pixelfox-dev
   (bei S3 wird das intern ohnehin auf s3://<bucket> gesetzt)
5. Public Base URL: https://images-b2.pixelfox.cc
6. Upload API URL: leer lassen
7. Node ID: leer lassen (oder b2-dev)
8. Access Key ID: <dein B2 keyID>
9. Secret Access Key: <dein B2 applicationKey>
10. Region: z. B. us-west-001 (genau aus B2 nehmen)
11. Bucket Name: pixelfox-dev
12. Endpoint URL: z. B. https://s3.us-west-001.backblazeb2.com (genau aus B2 S3-Infos)
13. Pfad-Präfix: leer lassen
    (wichtig, damit keine doppelten uploads/uploads/... entstehen)
14. Maximale Größe (GB): z. B. 5000
15. Priorität: z. B. 500 (niedrige Priorität, damit Hot/Warm vorher genutzt werden)
16. Aktiv: an
17. Standard-Pool: aus
18. Backup-Ziel (S3): aus (nur an, wenn dieser Pool auch Backup-Ziel sein soll)