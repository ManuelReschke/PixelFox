package imageprocessor_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ManuelReschke/PixelFox/app/models"
	"github.com/ManuelReschke/PixelFox/internal/pkg/imageprocessor"
)

// Struktur für einen Mock-Cache für unsere Tests
type mockCache struct {
	cache      map[string]string
	timestamps map[string]string
	mux        sync.Mutex
	setCalls   int
	getCalls   int
	delCalls   int
	// Für Datenbankaktualisierung
	updateRecordCalls int
}

var testCache = &mockCache{
	cache:      make(map[string]string),
	timestamps: make(map[string]string),
}

// Löscht den Cache für einen neuen Test
func (m *mockCache) clear() {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.cache = make(map[string]string)
	m.timestamps = make(map[string]string)
	m.setCalls = 0
	m.getCalls = 0
	m.delCalls = 0
	m.updateRecordCalls = 0
}

// Mock-Implementierung von Set
func (m *mockCache) set(key string, value interface{}, ttl time.Duration) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.setCalls++

	// Konvertiere value zu string
	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case int:
		strValue = fmt.Sprintf("%d", v)
	case bool:
		strValue = fmt.Sprintf("%v", v)
	default:
		strValue = fmt.Sprintf("%v", v)
	}

	m.cache[key] = strValue
	return nil
}

// Mock-Implementierung von Get
func (m *mockCache) get(key string) (string, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.getCalls++

	value, exists := m.cache[key]
	if !exists {
		return "", fmt.Errorf("cache: key not found")
	}
	return value, nil
}

// Mock-Implementierung von GetInt
func (m *mockCache) getInt(key string) (int, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.getCalls++

	value, exists := m.cache[key]
	if !exists {
		return 0, fmt.Errorf("cache: key not found")
	}

	i, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("cache: value is not an integer: %v", err)
	}
	return i, nil
}

// Mock-Implementierung von Delete
func (m *mockCache) delete(key string) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.delCalls++

	delete(m.cache, key)
	return nil
}

// Mock-Implementierung von updateImageRecord
func (m *mockCache) updateImageRecord(imageModel *models.Image, width, height int, hasWebp, hasAvif, hasThumbSmall, hasThumbMedium bool) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.updateRecordCalls++

	// Aktualisiere das Image-Model direkt
	imageModel.Width = width
	imageModel.Height = height
	imageModel.HasWebp = hasWebp
	imageModel.HasAVIF = hasAvif
	imageModel.HasThumbnailSmall = hasThumbSmall
	imageModel.HasThumbnailMedium = hasThumbMedium

	// Protokollieren
	fmt.Printf("[MockDB] Updated image %s: w=%d, h=%d, webp=%v, avif=%v, thumbS=%v, thumbM=%v\n",
		imageModel.UUID, width, height, hasWebp, hasAvif, hasThumbSmall, hasThumbMedium)

	return nil
}

// Installiere die Mock-Cache-Funktionen für den Test
func setupMockCache() func() {
	// Originale Implementierungen speichern
	originalSet := imageprocessor.SetCacheImplementation
	originalGet := imageprocessor.GetCacheImplementation
	originalGetInt := imageprocessor.GetIntCacheImplementation
	originalDelete := imageprocessor.DeleteCacheImplementation
	originalUpdateImageRecord := imageprocessor.UpdateImageRecordFunc

	// Mock-Implementierungen einsetzen
	imageprocessor.SetCacheImplementation = testCache.set
	imageprocessor.GetCacheImplementation = testCache.get
	imageprocessor.GetIntCacheImplementation = testCache.getInt
	imageprocessor.DeleteCacheImplementation = testCache.delete
	imageprocessor.UpdateImageRecordFunc = testCache.updateImageRecord

	// Test-Cleanup-Funktion zurückgeben, die Original-Implementierungen wiederherstellt
	return func() {
		imageprocessor.SetCacheImplementation = originalSet
		imageprocessor.GetCacheImplementation = originalGet
		imageprocessor.GetIntCacheImplementation = originalGetInt
		imageprocessor.DeleteCacheImplementation = originalDelete
		imageprocessor.UpdateImageRecordFunc = originalUpdateImageRecord
	}
}

// TestProcessImages überprüft, dass alle Bildtypen im testdata-Verzeichnis
// korrekt verarbeitet werden und alle erwarteten Varianten erstellt werden.
func TestProcessImages(t *testing.T) {
	// Test-Setup
	restoreCache := setupMockCache()
	defer restoreCache()

	// Arbeitsverzeichnis im Docker-Container ist /app
	workingDir, _ := os.Getwd()
	t.Logf("Test wird in Docker ausgeführt. Arbeitsverzeichnis: %s", workingDir)

	// Liste der Testbilder, die wir verarbeiten werden
	testImages := []string{
		"image.png", // PNG-Format
		"image-small.jpg",
		"image-big.jpg",
		"image-with-meta-data.jpg",
		"image.bmp",
		// "image.gif",
		// "image.svg",
	}

	// Eine Liste von Verzeichnissen, die wir am Ende aufräumen müssen
	cleanupDirs := []string{}

	// Aufräumfunktion, die am Ende des Tests ausgeführt wird
	defer func() {
		// Die erstellten temporären Verzeichnisse aufräumen
		for _, dir := range cleanupDirs {
			t.Logf("Räume Verzeichnis auf: %s", dir)
			err := os.RemoveAll(dir)
			if err != nil {
				t.Logf("Fehler beim Aufräumen von %s: %v", dir, err)
			}
		}

		// Aufräumen der VariantsDir Struktur
		variantsDir := imageprocessor.VariantsDir

		// Zuerst die Inhalte löschen
		files, _ := filepath.Glob(filepath.Join(variantsDir, "*"))
		for _, file := range files {
			t.Logf("Räume Variants-Unterverzeichnis auf: %s", file)
			os.RemoveAll(file)
		}

		// Dann den Variants-Ordner selbst löschen
		t.Logf("Räume Variants-Hauptverzeichnis auf: %s", variantsDir)
		os.RemoveAll(variantsDir)

		// Auch OriginalDir löschen, falls vorhanden
		originalDir := imageprocessor.OriginalDir
		if _, err := os.Stat(originalDir); err == nil {
			t.Logf("Räume Original-Verzeichnis auf: %s", originalDir)
			os.RemoveAll(originalDir)
		}
	}()

	// Processor für den Test initialisieren
	imageprocessor.GetProcessor().Start()
	defer imageprocessor.GetProcessor().Stop()

	// Jeden Test als Subtest ausführen
	for _, testFileName := range testImages {
		// Subtest für jedes Bild
		t.Run(testFileName, func(t *testing.T) {
			// Cache für jeden Test zurücksetzen
			testCache.clear()
			testCache.setCalls = 0
			testCache.getCalls = 0
			testCache.delCalls = 0
			testCache.updateRecordCalls = 0

			t.Logf("Verarbeite Testbild: %s", testFileName)

			// Eine eindeutige UUID für dieses Bild erstellen
			imageUUID := uuid.New().String()

			// Verzeichnis für dieses Testbild erstellen
			imageDir := imageUUID
			cleanupDirs = append(cleanupDirs, imageDir)

			// Erstelle das Verzeichnis, falls es nicht existiert
			os.MkdirAll(imageDir, 0755)

			// Pfad zum Original-Testbild
			testFilePath := filepath.Join("testdata", testFileName)

			// Ziel-Pfad im imageprocessor.OriginalDir
			targetFilePath := filepath.Join(imageDir, testFileName)
			t.Logf("Kopiere Testbild von %s nach %s", testFilePath, targetFilePath)

			// Testbild kopieren
			err := copyFile(t, testFilePath, targetFilePath)
			if err != nil {
				t.Fatalf("Fehler beim Kopieren der Testdatei: %v", err)
			}
			t.Logf("Testbild erfolgreich kopiert nach: %s", targetFilePath)

			// Dateiinformationen überprüfen
			fi, err := os.Stat(targetFilePath)
			if err == nil {
				t.Logf("Dateirechte: %v, Größe: %d Bytes", fi.Mode(), fi.Size())
			}

			// Image-Model erstellen
			imageType := filepath.Ext(testFileName)
			imageModel := &models.Image{
				UUID:     imageUUID,
				FilePath: imageDir,
				FileName: testFileName,
				FileType: imageType,
			}

			// Debug-Info zum Image-Model
			t.Logf("Image-Model konfiguriert:")
			t.Logf(" - UUID: %s", imageModel.UUID)
			t.Logf(" - FilePath: %s", imageModel.FilePath)
			t.Logf(" - FileName: %s", imageModel.FileName)
			t.Logf(" - FileType: %s", imageModel.FileType)
			t.Logf(" - Erwarteter Image-Pfad im Imageprocessor: %s", filepath.Join(imageModel.FilePath, imageModel.FileName))

			// Image zur Verarbeitung in die Warteschlange einreihen
			err = processImage(t, imageModel)
			if err != nil {
				t.Fatalf("Fehler bei der Bildverarbeitung: %v", err)
			}

			// Cache-Nutzung prüfen
			t.Logf("Cache-Statistik für %s: set=%d, get=%d, delete=%d, updateRecord=%d",
				testFileName, testCache.setCalls, testCache.getCalls, testCache.delCalls, testCache.updateRecordCalls)
			assert.Greater(t, testCache.setCalls, 0, "Cache Set sollte aufgerufen werden")

			// Die erstellten Varianten überprüfen
			verifyVariants(t, imageModel, imageType)
		})
	}
}

// processImage verarbeitet ein Bild und erstellt alle Varianten direkt
func processImage(t *testing.T, imageModel *models.Image) error {
	t.Helper()

	// Initialen Status mit unserem gemockten Cache setzen
	err := imageprocessor.SetImageStatus(imageModel.UUID, imageprocessor.STATUS_PENDING)
	require.NoError(t, err, "Failed to set image status")

	// Bild mit dem tatsächlichen Bildprozessor verarbeiten
	err = imageprocessor.ProcessImage(imageModel)
	if err != nil {
		// Wenn die Verarbeitung fehlschlägt, Status auf failed setzen (wie das echte System)
		statusErr := imageprocessor.SetImageStatus(imageModel.UUID, imageprocessor.STATUS_FAILED)
		if statusErr != nil {
			t.Logf("Fehler beim Setzen des STATUS_FAILED in Cache: %v", statusErr)
		}

		// Ausführliche Fehlerinformationen für die Fehlersuche
		t.Logf("Fehler bei der Bildverarbeitung:")
		t.Logf(" - UUID: %s", imageModel.UUID)
		t.Logf(" - FilePath: %s", imageModel.FilePath)
		t.Logf(" - FileName: %s", imageModel.FileName)
		t.Logf(" - Vollständiger Fehlermeldung: %v", err)

		// Prüfen, ob die Originaldatei existiert
		// Wir prüfen beide möglichen Pfadvarianten: relativen und absoluten Pfad
		relativePath := filepath.Join(imageModel.FilePath, imageModel.FileName)
		absolutePath := filepath.Join(imageprocessor.OriginalDir, imageModel.FilePath, imageModel.FileName)

		// Relativer Pfad (wie vom Imageprocessor erwartet)
		if _, err := os.Stat(relativePath); os.IsNotExist(err) {
			t.Logf(" - FEHLER: Originaldatei existiert nicht am relativen Pfad: %s", relativePath)
		} else if err != nil {
			t.Logf(" - FEHLER: Zugriffsfehler auf Originaldatei (relativ): %v", err)
		} else {
			t.Logf(" - Originaldatei existiert am relativen Pfad: %s", relativePath)
		}

		// Absoluter Pfad (unser erwarteter Standort)
		if _, err := os.Stat(absolutePath); os.IsNotExist(err) {
			t.Logf(" - FEHLER: Originaldatei existiert nicht am absoluten Pfad: %s", absolutePath)
		} else if err != nil {
			t.Logf(" - FEHLER: Zugriffsfehler auf Originaldatei (absolut): %v", err)
		} else {
			t.Logf(" - Originaldatei existiert am absoluten Pfad: %s", absolutePath)
		}

		// Alle Dateien im Verzeichnis auflisten
		t.Logf("Auflistung aller Dateien im aktuellen Verzeichnis:")
		files, _ := filepath.Glob("*")
		for _, f := range files {
			t.Logf(" - %s", f)
		}

		t.Logf("Auflistung aller Dateien im Uploads-Verzeichnis:")
		files, _ = filepath.Glob("uploads/**/*")
		for _, f := range files {
			t.Logf(" - %s", f)
		}

		// Aktuelles Verzeichnis ausgeben
		wd, wdErr := os.Getwd()
		if wdErr == nil {
			t.Logf("Aktuelles Arbeitsverzeichnis: %s", wd)
		}

		return err
	}

	// Auf den Abschluss der asynchronen Verarbeitung warten
	t.Logf("Warte auf Abschluss der asynchronen Verarbeitung...")
	waitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for {
		status, getErr := imageprocessor.GetImageStatus(imageModel.UUID)
		if getErr != nil {
			t.Logf("Fehler beim Abrufen des Status: %v", getErr)
			return getErr
		}

		// Status für Debugging ausgeben
		t.Logf("Aktueller Status: %s", status)

		if status == imageprocessor.STATUS_COMPLETED {
			t.Logf("Bildverarbeitung abgeschlossen!")
			break
		} else if status == imageprocessor.STATUS_FAILED {
			return fmt.Errorf("Bildverarbeitung fehlgeschlagen mit Status %s", status)
		}

		// Prüfen, ob das Timeout erreicht wurde
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("Timeout beim Warten auf Bildverarbeitung")
		case <-time.After(100 * time.Millisecond):
			// Kurz warten und erneut prüfen
		}
	}

	// Verzeichnisinhalt anzeigen, um die tatsächlich erstellten Dateien zu sehen
	files, _ := filepath.Glob(filepath.Join(imageprocessor.VariantsDir, "**/*"))
	t.Logf("Erstelle Dateien im Varianten-Verzeichnis: %v", files)

	// Überprüfen des endgültigen Status
	finaleStatus, err := imageprocessor.GetImageStatus(imageModel.UUID)
	if err != nil {
		return fmt.Errorf("Fehler beim Abrufen des Status nach dem Warten: %v", err)
	}

	// Sicherstellen, dass der Status COMPLETED ist
	if finaleStatus != imageprocessor.STATUS_COMPLETED {
		return fmt.Errorf("Unerwarteter finaler Status: %s", finaleStatus)
	}

	return nil
}

// verifyVariants überprüft, dass alle erwarteten Bildvarianten für ein Bild existieren
func verifyVariants(t *testing.T, imageModel *models.Image, fileExt string) {
	t.Helper()

	// Formatnamen für die Log-Ausgabe
	formatMap := map[string]string{
		".png":  "PNG",
		".jpg":  "JPEG",
		".jpeg": "JPEG",
		".gif":  "GIF",
		".svg":  "SVG",
		".webp": "WebP",
		".avif": "AVIF",
		".bmp":  "BMP",
		".heic": "HEIC",
		".heif": "HEIF",
	}

	// Typ des Originalbildes bestimmen
	imageType := strings.ToLower(fileExt)
	formatName, exists := formatMap[imageType]
	if !exists {
		formatName = strings.ToUpper(strings.TrimPrefix(imageType, "."))
	}

	// Basisname für die Varianten
	baseName := imageModel.UUID
	relativePath := strings.TrimPrefix(imageModel.FilePath, imageprocessor.OriginalDir)
	relativePath = strings.TrimPrefix(relativePath, string(filepath.Separator))

	// Debug-Info zu Pfaden ausgeben
	t.Logf("Pfadinformationen:")
	t.Logf(" - UUID: %s", imageModel.UUID)
	t.Logf(" - FilePath: %s", imageModel.FilePath)
	t.Logf(" - OriginalDir: %s", imageprocessor.OriginalDir)
	t.Logf(" - VariantsDir: %s", imageprocessor.VariantsDir)
	t.Logf(" - RelativePath: %s", relativePath)

	// Die tatsächlichen Pfade konstruieren, wie sie vom Imageprocessor erstellt werden sollten
	// WICHTIG: Hier müssen wir die Basispfade aus dem Imageprocessor verwenden!
	variantsBaseDir := filepath.Join(imageprocessor.VariantsDir, relativePath)
	t.Logf(" - Variants-Basisdir: %s", variantsBaseDir)

	// Pfade für alle möglichen Varianten
	webpPath := filepath.Join(variantsBaseDir, baseName+".webp")
	avifPath := filepath.Join(variantsBaseDir, baseName+".avif")
	smallThumbWebPPath := filepath.Join(variantsBaseDir, baseName+"_small.webp")
	mediumThumbWebPPath := filepath.Join(variantsBaseDir, baseName+"_medium.webp")
	smallThumbAVIFPath := filepath.Join(variantsBaseDir, baseName+"_small.avif")
	mediumThumbAVIFPath := filepath.Join(variantsBaseDir, baseName+"_medium.avif")

	// Debug-Info zu erwarteten Dateipfaden
	t.Logf("Erwartete Dateipfade:")
	t.Logf(" - WebP: %s", webpPath)
	t.Logf(" - AVIF: %s", avifPath)
	t.Logf(" - Small WebP: %s", smallThumbWebPPath)
	t.Logf(" - Medium WebP: %s", mediumThumbWebPPath)
	t.Logf(" - Small AVIF: %s", smallThumbAVIFPath)
	t.Logf(" - Medium AVIF: %s", mediumThumbAVIFPath)

	// Debug: Suchen und Anzeigen aller vorhandenen Dateien im Varianten-Verzeichnis
	allFiles, _ := filepath.Glob(filepath.Join(imageprocessor.VariantsDir, "**/*.*"))
	t.Logf("Alle gefundenen Dateien im Variants-Verzeichnis:")
	for _, f := range allFiles {
		t.Logf(" - %s", f)
	}

	// Sonderbehandlung für bestimmte Eingabeformate
	isGif := imageType == ".gif"
	isSvg := imageType == ".svg"
	isAvif := imageType == ".avif"

	// Überprüfen, ob die erwarteten Dateien existieren
	// Keine WebP/AVIF Konvertierung für GIF, SVG oder AVIF-Eingaben
	if !isGif && !isSvg && !isAvif {
		// Vollversion WebP sollte existieren
		assert.FileExists(t, webpPath, "WebP-Version sollte für %s existieren", formatName)

		// AVIF nur, wenn ffmpeg verfügbar ist
		if imageprocessor.IsFFmpegAvailable {
			assert.FileExists(t, avifPath, "AVIF-Version sollte für %s existieren", formatName)
		}
	}

	// Thumbnails für alle außer SVG
	if !isSvg {
		assert.FileExists(t, smallThumbWebPPath, "Kleines WebP-Thumbnail sollte für %s existieren", formatName)
		assert.FileExists(t, mediumThumbWebPPath, "Mittleres WebP-Thumbnail sollte für %s existieren", formatName)

		// AVIF-Thumbnails nur, wenn ffmpeg verfügbar
		if imageprocessor.IsFFmpegAvailable {
			assert.FileExists(t, smallThumbAVIFPath, "Kleines AVIF-Thumbnail sollte für %s existieren", formatName)
			assert.FileExists(t, mediumThumbAVIFPath, "Mittleres AVIF-Thumbnail sollte für %s existieren", formatName)
		}
	}

	// Überprüfen, dass die Flags im Modell gesetzt sind
	if !isGif && !isSvg && !isAvif {
		assert.True(t, imageModel.HasWebp, "HasWebp-Flag sollte für %s true sein", formatName)
		if imageprocessor.IsFFmpegAvailable {
			assert.True(t, imageModel.HasAVIF, "HasAVIF-Flag sollte für %s true sein", formatName)
		}
	}

	// Thumbnails für alle außer SVG
	if !isSvg {
		assert.True(t, imageModel.HasThumbnailSmall, "HasThumbnailSmall-Flag sollte für %s true sein", formatName)
		assert.True(t, imageModel.HasThumbnailMedium, "HasThumbnailMedium-Flag sollte für %s true sein", formatName)
	}
}

// copyFile kopiert eine Datei von src nach dst
func copyFile(t *testing.T, src, dst string) error {
	t.Helper()

	// Quell- und Zielverzeichnisse erstellen
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("Fehler beim Erstellen des Zielverzeichnisses %s: %v", dstDir, err)
	}

	// Quelldatei öffnen
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("Fehler beim Öffnen der Quelldatei %s: %v", src, err)
	}
	defer source.Close()

	// Zieldatei erstellen
	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("Fehler beim Erstellen der Zieldatei %s: %v", dst, err)
	}
	defer destination.Close()

	// Inhalt kopieren
	bytes, err := io.Copy(destination, source)
	if err != nil {
		return fmt.Errorf("Fehler beim Kopieren des Inhalts von %s nach %s: %v", src, dst, err)
	}

	// Dateiattribute von der Quelldatei übernehmen
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("Fehler beim Abrufen der Quelldateiinformationen: %v", err)
	}

	if err = os.Chmod(dst, sourceInfo.Mode()); err != nil {
		t.Logf("Warnung: Konnte Dateirechte nicht übernehmen: %v", err)
	}

	t.Logf("Datei erfolgreich kopiert: %s -> %s (Größe: %d Bytes)", src, dst, bytes)
	return nil
}

// isImageFile prüft anhand der Dateierweiterung, ob eine Datei ein Bild ist
func isImageFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	validExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
		".avif": true,
		".bmp":  true,
		".svg":  true,
		".heic": true,
		".heif": true,
	}
	return validExts[ext]
}
