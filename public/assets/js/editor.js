// Initialize editor when DOM is loaded
document.addEventListener('DOMContentLoaded', initEditor);

// Also initialize when htmx content is swapped
document.addEventListener('htmx:afterSwap', function(event) {
    // Check if the swapped content contains our editor element
    if (event.detail.target.querySelector('#content')) {
        initEditor();
    }
});

// Store editor instances
let editorInstance = null;

function initEditor() {
    // Check if we have a content element on the page
    const contentElement = document.getElementById('content');
    if (contentElement) {
        // Clean up any existing editor instance
        if (editorInstance) {
            editorInstance.destroy().catch(error => {});
            editorInstance = null;
        }
        
        // Remove the required attribute from the textarea since CKEditor will take over
        contentElement.removeAttribute('required');
        
        // Load CKEditor if not already loaded
        if (typeof ClassicEditor === 'undefined') {
            var script = document.createElement('script');
            // Use the latest version of CKEditor 5 (Classic Editor build)
            script.src = 'https://cdn.ckeditor.com/ckeditor5/41.0.0/classic/ckeditor.js';
            script.onload = function() {
                createEditor(contentElement);
            };
            document.head.appendChild(script);
        } else {
            createEditor(contentElement);
        }

        // Add form submit event listener to update textarea before submission
        const form = contentElement.closest('form');
        if (form) {
            form.addEventListener('submit', function(e) {
                if (editorInstance) {
                    // Get data from editor and update the hidden textarea
                    const data = editorInstance.getData();
                    contentElement.value = data;
                    
                    // Custom validation if the textarea was originally required
                    if (data.trim() === '') {
                        e.preventDefault();
                        // Use SweetAlert2 instead of basic alert
                        Swal.fire({
                            title: 'Fehler',
                            text: 'Der Inhalt darf nicht leer sein.',
                            icon: 'error',
                            confirmButtonText: 'OK'
                        });
                        return false;
                    }
                }
            });
        }
    }
}

function createEditor(element) {
    // Initialize CKEditor with more features
    ClassicEditor
        .create(element, {
            toolbar: {
                items: [
                    'undo', 'redo',
                    '|', 'heading',
                    '|', 'bold', 'italic', 'strikethrough', 'underline',
                    '|', 'link', 'uploadImage', 'blockQuote', 'code', 'codeBlock',
                    '|', 'bulletedList', 'numberedList', 'todoList',
                    '|', 'outdent', 'indent',
                    '|', 'alignment',
                    '|', 'insertTable', 'horizontalLine',
                    '|', 'fontColor', 'fontBackgroundColor'
                    // '|', 'removeFormat' <- Entfernt, da es in der Standard-Build nicht verfügbar ist
                ],
                shouldNotGroupWhenFull: true
            },
            heading: {
                options: [
                    { model: 'paragraph', title: 'Paragraph', class: 'ck-heading_paragraph' },
                    { model: 'heading1', view: 'h1', title: 'Heading 1', class: 'ck-heading_heading1' },
                    { model: 'heading2', view: 'h2', title: 'Heading 2', class: 'ck-heading_heading2' },
                    { model: 'heading3', view: 'h3', title: 'Heading 3', class: 'ck-heading_heading3' },
                    { model: 'heading4', view: 'h4', title: 'Heading 4', class: 'ck-heading_heading4' }
                ]
            },
            image: {
                toolbar: [
                    'imageStyle:inline',
                    'imageStyle:block',
                    'imageStyle:side',
                    '|',
                    'toggleImageCaption',
                    'imageTextAlternative'
                ]
            },
            table: {
                contentToolbar: [
                    'tableColumn',
                    'tableRow',
                    'mergeTableCells',
                    'tableCellProperties',
                    'tableProperties'
                ]
            },
            language: 'de',
            // Increase the editor's height
            height: '500px'
        })
        .then(editor => {
            editorInstance = editor;
            
            // Store the editor instance
            element.editor = editor;
            
            // Add event listener for form submission
            const form = element.closest('form');
            if (form) {
                form.addEventListener('submit', function() {
                    element.value = editor.getData();
                });
            }
        })
        .catch(error => {
            console.error('CKEditor initialization error:', error);
            // Fallback to basic textarea if editor fails to load
            element.style.display = 'block';
        });
}
