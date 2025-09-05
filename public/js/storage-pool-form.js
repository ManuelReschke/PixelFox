// Storage Pool Form JavaScript
// Handles S3 configuration show/hide functionality

function initStoragePoolForm() {
    const storageTypeSelect = document.querySelector('select[name="storage_type"]');
    const s3Config = document.getElementById('s3-config');
    const basePathInput = document.querySelector('input[name="base_path"]');
    
    if (!storageTypeSelect || !s3Config || !basePathInput) {
        return;
    }
    
    // Avoid duplicate listeners on repeated HTMX swaps
    if (storageTypeSelect.getAttribute('data-pxf-storage-init') === '1') {
        // Still ensure the UI reflects the current state
        toggleS3Config();
        return;
    }

    function toggleS3Config() {
        const isS3 = storageTypeSelect.value === 's3';
        s3Config.style.display = isS3 ? 'block' : 'none';
        
        // Update base path placeholder for S3
        if (isS3) {
            basePathInput.placeholder = 'S3 Bucket Name (wird automatisch gesetzt)';
            basePathInput.readOnly = true;
            basePathInput.value = 's3://bucket-name';
        } else {
            basePathInput.placeholder = '/mnt/storage/images';
            basePathInput.readOnly = false;
            if (basePathInput.value.startsWith('s3://')) {
                basePathInput.value = '';
            }
        }
        
        // Set required attributes for S3 fields
        const s3RequiredFields = ['s3_access_key_id', 's3_secret_access_key', 's3_region', 's3_bucket_name'];
        s3RequiredFields.forEach(fieldName => {
            const field = document.querySelector(`input[name="${fieldName}"]`);
            if (field) {
                field.required = isS3;
            }
        });
        
        console.log('Toggled S3 config:', isS3 ? 'visible' : 'hidden');
    }
    
    // Initialize on page load
    toggleS3Config();
    
    // Listen for changes
    storageTypeSelect.addEventListener('change', toggleS3Config);
    storageTypeSelect.setAttribute('data-pxf-storage-init', '1');
    
    // initialized
}

// Initialize on full load
document.addEventListener('DOMContentLoaded', () => {
    requestAnimationFrame(initStoragePoolForm);
});

// Robust HTMX hooks
function reinitStoragePoolForm() {
    requestAnimationFrame(() => setTimeout(initStoragePoolForm, 30));
}
document.addEventListener('htmx:load', reinitStoragePoolForm);
document.addEventListener('htmx:afterSwap', reinitStoragePoolForm);
document.addEventListener('htmx:afterSettle', reinitStoragePoolForm);
