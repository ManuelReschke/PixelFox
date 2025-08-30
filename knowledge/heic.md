################### HEIC


Basierend auf meiner Analyse der aktuellen Image-Processing-Pipeline kann ich Ihnen eine detaillierte Einschätzung zur HEIC-Unterstützung geben:

Aktuelle Situation:

Die Pipeline nutzt hauptsächlich:
- github.com/disintegration/imaging für Standard-Bildformate (JPEG, PNG, GIF, WebP, BMP)
- ffmpeg für AVIF-Konvertierung und -Dimensionen
- github.com/kolesa-team/go-webp für WebP-Encoding

HEIC-Unterstützung: Machbar, aber mit Einschränkungen

✅ Was funktionieren würde:

1. Decoding: imaging.Open() kann HEIC nicht direkt lesen
2. Alternative: ffmpeg kann HEIC lesen und konvertieren
3. Processing: Alle Thumbnail-Generierung und Varianten-Erstellung würde funktionieren
4. Metadaten: EXIF-Extraktion könnte problematisch sein

⚠️ Erforderliche Änderungen:

1. Spezielle HEIC-Behandlung (ähnlich wie AVIF):
   isHEIC := strings.HasSuffix(lowerFilePath, ".heic") || strings.HasSuffix(lowerFilePath, ".heif")
   if isHEIC {
   // ffmpeg für Dimensionen und Konvertierung zu PNG/JPEG
   // Dann normale Pipeline
   }
2. ffmpeg-Abhängigkeit erweitern:
   - Momentan nur für AVIF-Output
   - Müsste für HEIC-Input erweitert werden
3. Zwei-Stufen-Prozess:
   - HEIC → PNG/JPEG (via ffmpeg)
   - Dann normale Pipeline

🔧 Implementierungsaufwand:

Gering bis Mittel - etwa 2-3 Stunden Arbeit:
- Neue HEIC-Erkennungslogik
- ffmpeg-Konvertierung für Input
- Anpassung der Fehlermeldungen
- Tests

📋 Empfehlung:

JA, HEIC-Support ist sinnvoll und machbar, weil:

1. Häufiges Format: Viele iPhone-Nutzer haben HEIC-Bilder
2. Bestehende Infrastruktur: ffmpeg ist bereits vorhanden
3. Geringe Komplexität: Ähnliche Implementierung wie AVIF-Input
4. Bessere UX: Statt Fehlermeldung → automatische Konvertierung

🚀 Umsetzungsvorschlag:

Soll ich HEIC-Unterstützung implementieren? Das würde bedeuten:
- HEIC in erlaubte Formate aufnehmen
- Spezielle HEIC-Behandlung im ImageProcessor
- ffmpeg-basierte Konvertierung zu PNG für weitere Verarbeitung
- Alle Thumbnails und Varianten würden normal funktionieren

Zeitaufwand: Ca. 2-3 Stunden
Risiko: Niedrig (ffmpeg ist bereits getestet)

Soll ich das umsetzen?