//custom js

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
    const words = ["schnelles", "sicheres", "anonymes"];
    let currentIndex = 0;
    
    // CSS für den Übergangseffekt
    wordElement.style.transition = "opacity 0.5s ease-in-out";
    
    // Funktion zum Ändern des Wortes mit Fade-Effekt
    function changeWord() {
        // Ausblenden
        wordElement.style.opacity = 0;
        
        setTimeout(() => {
            // Nächstes Wort wählen
            currentIndex = (currentIndex + 1) % words.length;
            wordElement.textContent = words[currentIndex];
            
            // Einblenden
            wordElement.style.opacity = 1;
        }, 500); // Halbe Sekunde zum Ausblenden
    }
    
    // Wort alle 3 Sekunden ändern
    setInterval(changeWord, 3000);
}

/**
 * Initialisiert die Upload-Formular-Funktionalität
 */
function initUploadForm() {
    const uploadForm = document.getElementById('upload_form');
    if (!uploadForm) return;
    
    // Fortschrittsbalken anzeigen, wenn die Anfrage konfiguriert wird
    htmx.on('#upload_form', 'htmx:configRequest', function(evt) {
        htmx.find('#progress').classList.remove('hidden');
    });

    // Fortschrittsbalken aktualisieren während des Uploads
    htmx.on('#upload_form', 'htmx:xhr:progress', function(evt) {
        htmx.find('#progress').setAttribute('value', evt.detail.loaded/evt.detail.total * 100);
    });

    // Fehlerbehandlung nach der Anfrage
    htmx.on('htmx:afterRequest', (evt) => {
        if (evt.detail.xhr.status === 413) {
            const errorMessage = document.getElementById('error-message');
            errorMessage.classList.add('showError');
            errorMessage.textContent = 'Die Datei ist zu groß.';
            errorMessage.classList.add('fade-out');
            setTimeout(function() {
                errorMessage.style.display = 'none';
            }, 500);
        }
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