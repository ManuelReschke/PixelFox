// PixelFox Image Viewer JavaScript
// Handles tab functionality, format selection, modal display, and meta info toggle

// Global variable to store image paths
if (typeof window.pixelFoxImagePaths === 'undefined') {
	window.pixelFoxImagePaths = {};
}

// Function to update global image paths
function updateImagePaths(paths) {
	window.pixelFoxImagePaths = paths;
}

// Function to get data from HTML data attributes
function getImageData() {
	const dataElement = document.getElementById('image-data');
	if (dataElement) {
		return {
			domain: dataElement.getAttribute('data-domain') || '',
			displayName: dataElement.getAttribute('data-display-name') || '',
			avifPath: dataElement.getAttribute('data-avif-path') || '',
			webpPath: dataElement.getAttribute('data-webp-path') || '',
			originalPath: dataElement.getAttribute('data-original-path') || '',
			hasAvif: dataElement.getAttribute('data-has-avif') === 'true',
			hasWebp: dataElement.getAttribute('data-has-webp') === 'true'
		};
	}
	return {
		domain: '',
		displayName: '',
		avifPath: '',
		webpPath: '',
		originalPath: '',
		hasAvif: false,
		hasWebp: false
	};
}

// Function to initialize image paths from data attributes
function initializeImagePaths() {
	const data = getImageData();
	updateImagePaths({
		avif: data.avifPath,
		webp: data.webpPath,
		original: data.originalPath,
		hasAvif: data.hasAvif,
		hasWebp: data.hasWebp,
		displayName: data.displayName
	});
}

// Function to open the modal and load images on demand
function openImageModal() {
	// Show the modal first
	const modal = document.getElementById('image-modal');
	modal.showModal();
	
	// Show loading spinner, hide picture
	const spinner = document.getElementById('loading-spinner');
	const picture = document.getElementById('modal-picture');
	spinner.classList.remove('hidden');
	picture.classList.add('hidden');
	
	// Get the image element
	const img = document.getElementById('modal-image');
	
	// Clear existing sources
	while (picture.firstChild) {
		if (picture.firstChild !== img) {
			picture.removeChild(picture.firstChild);
		} else {
			break;
		}
	}
	
	// Create image object to preload
	const preloadImg = new Image();
	
	// Determine which optimized format to use
	let imageSrc = "";
	// Check if AVIF is available and the path is valid
	if (window.pixelFoxImagePaths.hasAvif && window.pixelFoxImagePaths.avif && window.pixelFoxImagePaths.avif.trim() !== "") {
		// Use AVIF if available
		imageSrc = window.pixelFoxImagePaths.avif;
		const avifSource = document.createElement('source');
		avifSource.srcset = window.pixelFoxImagePaths.avif;
		avifSource.type = 'image/avif';
		picture.insertBefore(avifSource, img);
	// Check if WebP is available and the path is valid
	} else if (window.pixelFoxImagePaths.hasWebp && window.pixelFoxImagePaths.webp && window.pixelFoxImagePaths.webp.trim() !== "") {
		// Use WebP if AVIF not available
		imageSrc = window.pixelFoxImagePaths.webp;
		const webpSource = document.createElement('source');
		webpSource.srcset = window.pixelFoxImagePaths.webp;
		webpSource.type = 'image/webp';
		picture.insertBefore(webpSource, img);
	} else {
		// Fallback to original only if no optimized version exists or paths are invalid
		imageSrc = window.pixelFoxImagePaths.original;
	}
	
	// When image loads, hide spinner and show image
	preloadImg.onload = function() {
		spinner.classList.add('hidden');
		picture.classList.remove('hidden');
	};
	
	// Set image src and alt
	img.alt = window.pixelFoxImagePaths.displayName;
	img.src = imageSrc; // Use the selected optimized format
	preloadImg.src = imageSrc; // Start loading
}

// Function to initialize size tabs (Medium, Small, Original)
function initializeTabs() {
	// Tab-Elemente
	const tabs = {
		medium: document.getElementById('tab-medium'),
		small: document.getElementById('tab-small'),
		optimized: document.getElementById('tab-optimized')
	};
	
	// Tab-Inhalte
	const contents = {
		medium: document.getElementById('tab-content-medium'),
		small: document.getElementById('tab-content-small'),
		optimized: document.getElementById('tab-content-optimized')
	};
	
	// Abbruch, wenn nicht alle Elemente gefunden wurden
	if (!tabs.medium || !tabs.small || !tabs.optimized || 
	    !contents.medium || !contents.small || !contents.optimized) {
		return;
	}
	
	// Aktiviere einen Tab
	function activateTab(tabName) {
		// Deaktiviere alle Tabs
		Object.values(tabs).forEach(tab => tab.classList.remove('tab-active'));
		
		// Verstecke alle Inhalte (mit !important, um andere CSS zu überschreiben)
		Object.values(contents).forEach(content => {
			content.classList.add('hidden');
			content.style.display = 'none';
		});
		
		// Aktiviere den ausgewählten Tab
		tabs[tabName].classList.add('tab-active');
		
		// Zeige den ausgewählten Inhalt
		contents[tabName].classList.remove('hidden');
		contents[tabName].style.display = 'block';
	}
	
	// Event-Listener für Tabs
	tabs.medium.addEventListener('click', () => activateTab('medium'));
	tabs.small.addEventListener('click', () => activateTab('small'));
	tabs.optimized.addEventListener('click', () => activateTab('optimized'));
	
	// Initial Medium-Tab aktivieren
	activateTab('medium');
}

// Function to initialize format tab functionality
function initializeFormatTabs() {
	// Handle format tab selection for all sizes
	const formatTabs = document.querySelectorAll('[id^="format-tab-"]');
	formatTabs.forEach(tab => {
		tab.addEventListener('click', function(e) {
			e.preventDefault();
			
			const format = this.getAttribute('data-format');
			const size = this.getAttribute('data-size');
			const path = this.getAttribute('data-path');
			
			// Remove active class from all format tabs for this size
			const siblingTabs = document.querySelectorAll(`[id^="format-tab-${size}-"]`);
			siblingTabs.forEach(sibling => sibling.classList.remove('tab-active'));
			
			// Add active class to clicked tab
			this.classList.add('tab-active');
			
			// Update all input fields for this size
			updateInputFields(size, format, path);
		});
	});

	// Initialize input fields for all active tabs on page load
	const activeTabs = document.querySelectorAll('[id^="format-tab-"].tab-active');
	activeTabs.forEach(tab => {
		const format = tab.getAttribute('data-format');
		const size = tab.getAttribute('data-size');
		const path = tab.getAttribute('data-path');
		updateInputFields(size, format, path);
	});
}

// Function to update input fields when format changes
function updateInputFields(size, format, path) {
	const data = getImageData();
	const domain = data.domain;
	const displayName = data.displayName;
	
	// Escape any quotes in displayName to prevent XSS
	const safeDisplayName = displayName.replace(/"/g, '&quot;');
	
	// Update HTML input
	const htmlInput = document.getElementById(`html-${size}`);
	if (htmlInput) {
		htmlInput.value = `<img src="${domain}${path}" alt="${safeDisplayName}" />`;
	}
	
	// Update BBCode input  
	const bbcodeInput = document.getElementById(`bbcode-${size}`);
	if (bbcodeInput) {
		bbcodeInput.value = `[img]${domain}${path}[/img]`;
	}
	
	// Update Markdown input
	const markdownInput = document.getElementById(`markdown-${size}`);
	if (markdownInput) {
		markdownInput.value = `![${displayName}](${domain}${path})`;
	}
	
	// Update Direktlink input
	const direktlinkInput = document.getElementById(`direktlink-${size}`);
	if (direktlinkInput) {
		direktlinkInput.value = `${domain}${path}`;
	}
}

// Toggle Meta-Info initialization
function initToggleMeta() {
	const link = document.getElementById('toggle-meta');
	if (link) {
		// Remove existing event listeners to prevent duplicates
		const newLink = link.cloneNode(true);
		link.parentNode.replaceChild(newLink, link);
		
		newLink.addEventListener('click', function(e) {
			e.preventDefault();
			const info = document.getElementById('meta-info');
			const icon = document.getElementById('toggle-meta-icon');
			if (info) info.classList.toggle('hidden');
			if (icon) icon.classList.toggle('rotate-180');
		});
	}
}

// Function to initialize event listeners
function initializeEventListeners() {
	// Add click event to preview image
	const previewImage = document.getElementById('preview-image');
	if (previewImage) {
		// Remove any existing listeners to avoid duplicates
		previewImage.removeEventListener('click', openImageModal);
		// Add the click event listener
		previewImage.addEventListener('click', openImageModal);
	}

	// Update image paths when processed image element is loaded
	const processedImageElement = document.getElementById('processed-image-element');
	if (processedImageElement) {
		// Extract paths from data attributes
		updateImagePaths({
			avif: processedImageElement.getAttribute('data-avif-path') || '',
			webp: processedImageElement.getAttribute('data-webp-path') || '',
			original: processedImageElement.getAttribute('data-original-path') || '',
			hasAvif: processedImageElement.getAttribute('data-has-avif') === 'true',
			hasWebp: processedImageElement.getAttribute('data-has-webp') === 'true',
			displayName: processedImageElement.getAttribute('data-display-name') || ''
		});
	}

	// Initialize format tabs with slight delay to ensure DOM is ready
	setTimeout(initializeFormatTabs, 50);
}

// Initialize clipboard functionality
function initializeClipboard() {
	// Initialize clipboard
	var clipboard = new ClipboardJS('.copy-btn');
	
	clipboard.on('success', function(e) {
		const button = e.trigger;
		const originalHTML = button.innerHTML;
		
		button.innerHTML = `
			<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-4 h-4">
				<path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
			</svg>
		`;
		
		setTimeout(function() {
			button.innerHTML = originalHTML;
		}, 1500);
		
		e.clearSelection();
	});
	
	clipboard.on('error', function(e) {
		console.error('Error copying: ', e.action);
	});
}

// Main initialization function
function initImageViewer() {
	initializeImagePaths();
	initializeEventListeners();
	initToggleMeta();
	initializeClipboard();
	initializeTabs();
}

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', initImageViewer);

// Initialize after HTMX swaps (for dynamic content updates)
document.body.addEventListener('htmx:afterSwap', function(event) {
	// Small delay to ensure DOM is fully updated after HTMX swap
	setTimeout(function() {
		initializeImagePaths();
		initializeEventListeners();
		initToggleMeta();
		initializeTabs();
	}, 100);
});