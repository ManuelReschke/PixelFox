// Storage Pool Form JavaScript
// Handles S3 configuration show/hide functionality

function initStoragePoolForm() {
    const storageTypeSelect = document.querySelector('select[name="storage_type"]');
    const s3Config = document.getElementById('s3-config');
    const basePathInput = document.querySelector('input[name="base_path"]');
    
    if (!storageTypeSelect || !s3Config || !basePathInput) {
        console.log('Storage pool form elements not found, skipping initialization');
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
    
    console.log('Storage pool form initialized');
}

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', initStoragePoolForm);

// Initialize after HTMX swaps (for dynamic content)
document.body.addEventListener('htmx:afterSwap', function(event) {
    // Small delay to ensure DOM is fully updated
    setTimeout(initStoragePoolForm, 100);
});