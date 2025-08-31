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
    
    // Counter-Animation für Profile-Seite initialisieren
    initCounters();
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
    const directUploadEnabled = (uploadForm.dataset.directUpload || '').toLowerCase() === 'true';
    
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
                errorText.textContent = 'Nur Bildformate werden unterstützt (JPG, JPEG, PNG, GIF, WEBP, AVIF, BMP)';
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

    // Direct-to-Storage flow
    async function directUpload(file) {
        try {
            // Show progress UI
            progressContainer.classList.remove('hidden');
            uploadButton.disabled = true;
            uploadStatus.textContent = 'Session...';

            // 1) Request upload session
            const sessRes = await fetch('/api/v1/upload/sessions', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                credentials: 'include',
                body: JSON.stringify({ file_size: file.size })
            });
            if (!sessRes.ok) {
                throw new Error('Session-Fehler: ' + (await sessRes.text()));
            }
            const sess = await sessRes.json();
            if (!sess.upload_url || !sess.token) {
                throw new Error('Ungültige Session-Antwort');
            }

            // (hint removed)

            // 2) Upload directly to storage using XHR for progress
            const uploadResultData = await new Promise((resolve, reject) => {
                const xhr = new XMLHttpRequest();
                xhr.open('POST', sess.upload_url, true);
                xhr.setRequestHeader('Authorization', 'Bearer ' + sess.token);
                xhr.upload.onprogress = (evt) => {
                    if (evt.lengthComputable) {
                        const percent = Math.round((evt.loaded / evt.total) * 100);
                        progressBar.style.width = percent + '%';
                        uploadPercentage.textContent = percent + '%';
                        if (percent === 100) uploadStatus.textContent = 'Verarbeitung...';
                    }
                };
                xhr.onreadystatechange = function() {
                    if (xhr.readyState === 4) {
                        if (xhr.status >= 200 && xhr.status < 300) {
                            try {
                                const data = JSON.parse(xhr.responseText || '{}');
                                resolve(data);
                            } catch(_) {
                                resolve({});
                            }
                        } else {
                            reject({ status: xhr.status, body: xhr.responseText });
                        }
                    }
                };
                const fd = new FormData();
                fd.append('file', file);
                xhr.send(fd);
            });

            // Parse response of last XHR? Some servers return the body on xhr.responseText; we try JSON parse.
            // We cannot reliably access it here due to Promise resolution; do a quick fetch to /image viewer by polling can be added later.
            // For now, hide progress and show success and reload to home where flash shows redirect.
            successMessage.classList.remove('hidden');
            uploadResult.classList.remove('hidden');
            uploadStatus.textContent = '';
            const isDuplicate = !!(uploadResultData && uploadResultData.duplicate);
            const maybeUUID = (uploadResultData && uploadResultData.image_uuid) || '';
            const maybeView = (uploadResultData && uploadResultData.view_url) || '';
            if (isDuplicate && maybeView) {
                // Redirect via flash helper to show info message
                window.location.href = '/flash/upload-duplicate?view=' + encodeURIComponent(maybeView);
                return;
            }
            if (maybeUUID) {
                uploadStatus.textContent = 'Verarbeitung...';
                let attempts = 0;
                const poll = setInterval(async () => {
                    attempts++;
                    try {
                        const r = await fetch(`/api/v1/image/status/${maybeUUID}`, { credentials: 'include' });
                        if (r.ok) {
                            const js = await r.json();
                            if (js.complete) {
                                clearInterval(poll);
                                const dest = js.view_url || maybeView || '/user/images';
                                window.location.href = dest;
                            }
                        }
                    } catch(_) {}
                    if (attempts > 120) { // ~3 Minuten
                        clearInterval(poll);
                        window.location.href = maybeView || '/user/images';
                    }
                }, 1500);
            } else if (maybeView) {
                window.location.href = maybeView;
            } else {
                window.location.href = '/user/images';
            }
            } catch (err) {
            // Map rate-limit errors
            if (err && err.status === 429) {
                window.location.href = '/flash/upload-rate-limit';
                return;
            }
            if (err && err.status === 413) {
                window.location.href = '/flash/upload-too-large';
                return;
            }
            if (err && err.status === 415) {
                window.location.href = '/flash/upload-unsupported-type';
                return;
            }
            // Try to extract error message and show as flash
            try {
                let msg = '';
                if (err && err.body) {
                    try { const obj = JSON.parse(err.body); msg = (obj && obj.error) || ''; } catch(_) {}
                }
                if (msg) {
                    window.location.href = '/flash/upload-error?msg=' + encodeURIComponent(msg);
                    return;
                }
            } catch(_e) {}
            // Fallback to original App upload
            console.error('Direct upload failed, fallback to App upload:', err);
            progressContainer.classList.add('hidden');
            try { uploadForm.removeAttribute('data-direct-upload'); } catch(e) {}
            uploadForm.submit();
        }
    }

    // Intercept submit when direct upload enabled
    uploadForm.addEventListener('submit', function(e) {
        if (!directUploadEnabled) return; // let HTMX handle
        e.preventDefault();
        if (!fileInput.files.length) return;
        directUpload(fileInput.files[0]);
    });

    // HTMX Event-Listener (fallback / non-direct mode)
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
        uploadResult.classList.add('hidden');
        
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

/**
 * Initialisiert die Counter-Animation für die Profil-Seite
 */
function initCounters() {
    const counters = document.querySelectorAll('.counter');
    if (!counters.length) return;
    
    counters.forEach(counter => {
        // Skip if already animated
        if (counter.getAttribute('data-animated') === 'true') {
            return;
        }
        
        const target = parseInt(counter.getAttribute('data-target'));
        const increment = target / 50;
        let current = 0;
        
        // Mark as animated to prevent duplicate animations
        counter.setAttribute('data-animated', 'true');
        
        const timer = setInterval(() => {
            current += increment;
            counter.textContent = Math.floor(current);
            
            if (current >= target) {
                counter.textContent = target;
                clearInterval(timer);
            }
        }, 50);
    });
}

// SweetAlert2 confirm for deleting a storage pool (admin)
function confirmDelete(poolId, poolName) {
    const name = poolName || '';
    Swal.fire({
        title: `Speicherpool "${name}" löschen?`,
        text: 'Dieser Vorgang kann nicht rückgängig gemacht werden.',
        icon: 'warning',
        showCancelButton: true,
        confirmButtonText: 'Ja, löschen',
        cancelButtonText: 'Abbrechen'
    }).then((result) => {
        if (result.isConfirmed) {
            window.location.href = `/admin/storage/delete/${poolId}`;
        }
    });
}

// Delegate click on gallery view buttons
document.addEventListener('click', (e) => {
    const btn = e.target.closest('.image-view-btn');
    if (!btn) return;
    e.preventDefault();

    // Collect all view buttons to build an ordered image list
    const buttons = Array.from(document.querySelectorAll('.image-view-btn'));
    const images = buttons.map(b => b.dataset.imageSrc);
    let currentIndex = buttons.indexOf(btn);

    // Helper to open the modal with navigation arrows
    const openImageModal = () => {
        Swal.fire({
            html: `
                <div class="relative flex justify-center items-center">
                    <button type="button" class="nav-btn prev-btn left-4 top-1/2 -translate-y-1/2 bg-black/50 hover:bg-black text-white rounded-full w-10 h-10 grid place-items-center cursor-pointer z-10">&#10094;</button>
                    <img src="${images[currentIndex]}" alt="Bild" class="modal-image max-h-[80vh] w-auto mx-auto"/>
                    <button type="button" class="nav-btn next-btn right-4 top-1/2 -translate-y-1/2 bg-black/50 hover:bg-black text-white rounded-full w-10 h-10 grid place-items-center cursor-pointer z-10">&#10095;</button>
                </div>`,
            showConfirmButton: false,
            showCloseButton: true,
            background: 'rgba(0,0,0,0.9)',
            width: '90%',
            padding: '1rem',
            didOpen: (popup) => {
                console.log('Image modal opened', { currentIndex, src: images[currentIndex] });
                const imgEl = popup.querySelector('.modal-image');
                const prevBtn = popup.querySelector('.prev-btn');
                const nextBtn = popup.querySelector('.next-btn');

                const updateImage = () => {
                    const newSrc = images[currentIndex];
                    console.log('Update image to', newSrc);
                    imgEl.src = newSrc;
                };

                if(prevBtn){
                  prevBtn.addEventListener('click', (ev) => {
                    ev.stopPropagation();
                    console.log('Prev arrow clicked');
                    currentIndex = (currentIndex - 1 + images.length) % images.length;
                    updateImage();
                  });
                }

                if(nextBtn){
                  nextBtn.addEventListener('click', (ev) => {
                    ev.stopPropagation();
                    console.log('Next arrow clicked');
                    currentIndex = (currentIndex + 1) % images.length;
                    updateImage();
                  });
                }
            }
        });
    };

    openImageModal();
});

// Kept for potential external usage (opens single image without arrows)
function viewImage(src) {
    Swal.fire({
        html: `<img src="${src}" alt="Bild" class="max-h-[80vh] w-auto mx-auto"/>`,
        showConfirmButton: false,
        showCloseButton: true,
        background: 'rgba(0,0,0,0.9)',
        width: '90%',
        padding: '1rem',
    });
}
