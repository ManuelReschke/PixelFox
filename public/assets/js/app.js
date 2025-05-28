//custom js

// Sofort das gespeicherte Theme anwenden, bevor die Seite gerendert wird
(function() {
    var savedTheme = localStorage.getItem('theme');
    if (savedTheme) {
        document.documentElement.setAttribute('data-theme', savedTheme);
    }
})();

document.addEventListener('DOMContentLoaded', () => {
    initializeAllFunctions();
});

// Initialisiere alle Funktionen
function initializeAllFunctions() {
    // Flash-Nachrichten ausblenden
    handleFlashMessages();
    
    // Dynamischen Wortwechsel initialisieren
    initChangingWords();
    
    // Upload-Formular-Funktionalität initialisieren
    initUploadForm();
    
    // Theme-Funktionalität initialisieren
    initThemeToggle();
}

// HTMX-Event-Listener für Seitenwechsel
document.addEventListener('htmx:afterSwap', function(event) {
    // Nach jedem HTMX-Seitenwechsel die Funktionen neu initialisieren
    initializeAllFunctions();
});

/**
 * Flash-Nachrichten mit Fade-Effekt ausblenden
 */
function handleFlashMessages() {
    setTimeout(function() {
        var flashMessage = document.getElementById('flash-message');
        if (flashMessage) {
            flashMessage.classList.add('fade-out');
            setTimeout(function() {
                flashMessage.style.display = 'none';
            }, 500); // Verstecke nach der Animation
        }
    }, 4000); // 4 Sekunden warten
}

/**
 * Initialisiert den dynamischen Wortwechsel für die Startseite
 */
function initChangingWords() {
    const wordElement = document.getElementById("changing-word");
    if (!wordElement) return;
    
    // Array mit den Wörtern, die angezeigt werden sollen
    const words = ["schnelles", "sicheres", "anonymes", "Made in Germany", "kostenloses", "einfaches", "zuverlässiges"];
    let currentIndex = 0;
    
    // CSS für den Übergangseffekt - verbesserte Animation
    wordElement.style.transition = "opacity 0.8s ease, transform 0.6s ease";
    wordElement.style.opacity = 1;
    
    // Initiales Wort setzen
    wordElement.textContent = words[currentIndex];
    
    // Funktion zum Ändern des Wortes mit verbessertem Fade-Effekt
    function changeWord() {
        // Ausblenden mit leichter Bewegung nach unten
        wordElement.style.opacity = 0;
        wordElement.style.transform = "translateY(10px)";
        
        setTimeout(() => {
            // Nächstes Wort wählen
            currentIndex = (currentIndex + 1) % words.length;
            wordElement.textContent = words[currentIndex];
            
            // Position für Einblendeffekt zurücksetzen
            wordElement.style.transform = "translateY(-10px)";
            
            // Kurze Verzögerung für besseren visuellen Effekt
            setTimeout(() => {
                // Einblenden mit Bewegung nach oben
                wordElement.style.opacity = 1;
                wordElement.style.transform = "translateY(0)";
            }, 50);
        }, 600); // Etwas mehr Zeit zum Ausblenden für flüssigeren Übergang
    }
    
    // Wort alle 4 Sekunden ändern für mehr Lesezeit
    setInterval(changeWord, 4000);
}

/**
 * Initialisiert die Upload-Formular-Funktionalität
 */
function initUploadForm() {
    const uploadForm = document.getElementById('upload_form');
    if (!uploadForm) return;
    
    const fileInput = document.getElementById('file-input');
    const dropArea = document.getElementById('drop-area');
    const fileName = document.getElementById('file-name');
    const uploadButton = document.getElementById('upload-button');
    const uploadIcon = document.getElementById('upload-icon');
    const inlineImagePreview = document.getElementById('inline-image-preview');
    const progressContainer = document.getElementById('progress-container');
    const progressBar = document.getElementById('progress-bar');
    const uploadPercentage = document.getElementById('upload-percentage');
    const uploadStatus = document.getElementById('upload-status');
    const uploadResult = document.getElementById('upload-result');
    const successMessage = document.getElementById('success-message');
    const errorMessage = document.getElementById('error-message');
    const successText = document.getElementById('success-text');
    const errorText = document.getElementById('error-text');

    // Datei-Input-Event-Listener
    fileInput.addEventListener('change', function() {
        if (this.files.length > 0) {
            const file = this.files[0];
            
            // Prüfe, ob es sich um ein Bild handelt
            if (!file.type.startsWith('image/')) {
                errorMessage.classList.remove('hidden');
                errorText.textContent = 'Nur Bildformate werden unterstützt (JPG, PNG, GIF, WEBP, SVG, BMP)';
                uploadResult.classList.remove('hidden');
                fileInput.value = ''; // Dateiauswahl zurücksetzen
                return;
            }
            
            fileName.textContent = file.name;
            uploadButton.disabled = false;
            dropArea.classList.add('border-primary');
            dropArea.classList.remove('border-primary/50');
            
            // Zeige Bildvorschau
            const reader = new FileReader();
            reader.onload = function(e) {
                uploadIcon.classList.add('hidden');
                inlineImagePreview.src = e.target.result;
                inlineImagePreview.classList.remove('hidden');
            };
            reader.readAsDataURL(file);
        } else {
            resetUploadForm();
        }
    });

    // Drag & Drop Funktionalität
    ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
        dropArea.addEventListener(eventName, preventDefaults, false);
    });

    function preventDefaults(e) {
        e.preventDefault();
        e.stopPropagation();
    }

    ['dragenter', 'dragover'].forEach(eventName => {
        dropArea.addEventListener(eventName, highlight, false);
    });

    ['dragleave', 'drop'].forEach(eventName => {
        dropArea.addEventListener(eventName, unhighlight, false);
    });

    function highlight() {
        dropArea.classList.add('border-primary');
        dropArea.classList.remove('border-primary/50');
    }

    function unhighlight() {
        if (!fileInput.files.length) {
            dropArea.classList.add('border-primary/50');
            dropArea.classList.remove('border-primary');
        }
    }

    dropArea.addEventListener('drop', handleDrop, false);

    function handleDrop(e) {
        const dt = e.dataTransfer;
        const files = dt.files;
        fileInput.files = files;
        
        // Trigger change event manually
        const event = new Event('change');
        fileInput.dispatchEvent(event);
    }

    // Formular zurücksetzen
    function resetUploadForm() {
        fileName.textContent = 'Datei hierher ziehen oder klicken zum Auswählen';
        uploadButton.disabled = true;
        dropArea.classList.add('border-primary/50');
        dropArea.classList.remove('border-primary');
        progressContainer.classList.add('hidden');
        uploadResult.classList.add('hidden');
        successMessage.classList.add('hidden');
        errorMessage.classList.add('hidden');
        progressBar.style.width = '0%';
        uploadPercentage.textContent = '0%';
        uploadIcon.classList.remove('hidden');
        inlineImagePreview.classList.add('hidden');
        inlineImagePreview.src = '';
    }

    // HTMX Event-Listener
    // Anfrage wird konfiguriert
    htmx.on('#upload_form', 'htmx:configRequest', function(evt) {
        progressContainer.classList.remove('hidden');
        uploadButton.disabled = true;
        uploadStatus.textContent = 'Wird hochgeladen...';
        // Füge HX-Request Header hinzu, um zu kennzeichnen, dass es sich um eine HTMX-Anfrage handelt
        evt.detail.headers['HX-Request'] = 'true';
        
        // Verstecke vorherige Ergebnisse beim Start eines neuen Uploads
        uploadResult.classList.add('hidden');
        successMessage.classList.add('hidden');
        errorMessage.classList.add('hidden');
    });

    // Fortschritt beim Upload
    htmx.on('#upload_form', 'htmx:xhr:progress', function(evt) {
        if (evt.lengthComputable) {
            const percentComplete = Math.round((evt.loaded / evt.total) * 100);
            progressBar.style.width = percentComplete + '%';
            uploadPercentage.textContent = percentComplete + '%';
            if (percentComplete === 100) {
                uploadStatus.textContent = 'Verarbeitung...';
            }
        }
    });

    // Erfolgreicher Upload
    htmx.on('#upload_form', 'htmx:beforeOnLoad', function(evt) {
        // Prüfe, ob ein Redirect Header gesetzt wurde
        const redirectHeader = evt.detail.xhr.getResponseHeader('HX-Redirect');
        if (redirectHeader) {
            // Wenn ein Redirect Header gesetzt wurde, wird die Seite automatisch umgeleitet
            // Wir müssen hier nichts tun
            return;
        }
    });

    // Fehler beim Upload
    htmx.on('#upload_form', 'htmx:responseError', function(evt) {
        // Fortschrittsanzeige zurücksetzen
        progressContainer.classList.add('hidden');
        progressBar.style.width = '0%';
        uploadPercentage.textContent = '0%';
        uploadStatus.textContent = '';
        
        // Fehlermeldung anzeigen
        errorMessage.classList.remove('hidden');
        errorText.textContent = evt.detail.xhr.responseText || 'Fehler beim Hochladen: Unbekannter Fehler';
        uploadResult.classList.remove('hidden');
        
        // Upload-Button wieder aktivieren
        uploadButton.disabled = false;
    });

    // Bei allen anderen Fehlern (z.B. Netzwerkfehler)
    htmx.on('#upload_form', 'htmx:sendError', function(evt) {
        // Fortschrittsanzeige zurücksetzen
        progressContainer.classList.add('hidden');
        progressBar.style.width = '0%';
        uploadPercentage.textContent = '0%';
        uploadStatus.textContent = '';
        
        // Fehlermeldung anzeigen
        errorMessage.classList.remove('hidden');
        errorText.textContent = 'Netzwerkfehler beim Hochladen. Bitte versuche es später erneut.';
        uploadResult.classList.remove('hidden');
        
        // Upload-Button wieder aktivieren
        uploadButton.disabled = false;
    });

    // Bei Abbruch des Uploads
    htmx.on('#upload_form', 'htmx:abort', function(evt) {
        // Fortschrittsanzeige zurücksetzen
        progressContainer.classList.add('hidden');
        progressBar.style.width = '0%';
        uploadPercentage.textContent = '0%';
        uploadStatus.textContent = '';
        
        // Upload-Button wieder aktivieren
        uploadButton.disabled = false;
    });
}

/**
 * Initialisiert die Theme-Toggle-Funktionalität
 */
function initThemeToggle() {
    const themeToggle = document.getElementById('theme-toggle');
    if (!themeToggle) return;
    
    // Beim Laden der Seite den gespeicherten Theme-Status abrufen
    const savedTheme = localStorage.getItem('theme');
    const htmlElement = document.documentElement;
    
    // Prüfen, ob das aktuelle Theme dunkel ist
    const isDarkMode = savedTheme === 'dark';
    
    // Setze das Theme entsprechend dem gespeicherten Wert
    if (savedTheme) {
        htmlElement.setAttribute('data-theme', savedTheme);
    } else {
        // Standardmäßig auf emerald setzen, wenn kein Theme gespeichert ist
        htmlElement.setAttribute('data-theme', 'emerald');
    }
    
    // Toggle-Schalter auf den richtigen Zustand setzen
    themeToggle.checked = isDarkMode;
    
    // Event-Listener für den Toggle-Schalter
    themeToggle.addEventListener('change', function() {
        // Wenn der Schalter aktiviert ist, setze das Theme auf dark, sonst auf emerald
        const newTheme = this.checked ? 'dark' : 'emerald';
        htmlElement.setAttribute('data-theme', newTheme);
        localStorage.setItem('theme', newTheme);
    });
}