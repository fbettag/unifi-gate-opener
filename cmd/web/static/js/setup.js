let currentStep = 1;
const totalSteps = 5; // Added site selection step
let setupData = {
    admin: {},
    unifi: {},
    shelly: {},
    gate: {}
};
let availableSites = [];

function showStep(step) {
    // Hide all steps
    document.querySelectorAll('.step').forEach(s => s.classList.add('hidden'));
    
    // Show current step
    document.getElementById(`step-${step}`).classList.remove('hidden');
    
    // Update navigation buttons
    document.getElementById('btn-previous').classList.toggle('hidden', step === 1);
    document.getElementById('btn-next').classList.toggle('hidden', step === totalSteps);
    document.getElementById('btn-finish').classList.toggle('hidden', step !== totalSteps);
    
    // Load sites on step 3
    if (step === 3) {
        loadSites();
    }
    // Load access points on step 4
    if (step === 4) {
        loadAccessPoints();
    }
}

async function nextStep() {
    if (await validateStep(currentStep)) {
        saveStepData(currentStep);
        currentStep++;
        showStep(currentStep);
    }
}

function previousStep() {
    currentStep--;
    showStep(currentStep);
}

async function validateStep(step) {
    switch (step) {
        case 1:
            const username = document.getElementById('admin-username').value;
            const password = document.getElementById('admin-password').value;
            const confirm = document.getElementById('admin-password-confirm').value;
            
            if (!username || !password) {
                alert('Please fill in all fields');
                return false;
            }
            
            if (password !== confirm) {
                alert('Passwords do not match');
                return false;
            }
            
            if (password.length < 8) {
                alert('Password must be at least 8 characters');
                return false;
            }
            
            return true;
            
        case 2:
            const url = document.getElementById('unifi-url').value;
            const unifiUser = document.getElementById('unifi-username').value;
            const unifiPass = document.getElementById('unifi-password').value;
            
            if (!url || !unifiUser || !unifiPass) {
                alert('Please fill in all UniFi fields');
                return false;
            }
            
            try {
                new URL(url);
            } catch {
                alert('Please enter a valid URL');
                return false;
            }
            
            // Test UniFi connection before proceeding
            const testBtn = document.getElementById('btn-next');
            testBtn.disabled = true;
            testBtn.innerHTML = '<i class="fas fa-spinner fa-spin mr-2"></i>Testing Connection...';
            
            try {
                const response = await fetch('/api/test-unifi', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        controller_url: url,
                        username: unifiUser,
                        password: unifiPass,
                        site_id: 'default' // Test with default site first
                    })
                });
                
                // Check if we got JSON response
                const contentType = response.headers.get('content-type');
                if (!contentType || !contentType.includes('application/json')) {
                    // If not JSON, we probably got redirected or received HTML
                    const text = await response.text();
                    console.error('Expected JSON but got:', contentType, text.substring(0, 100));
                    throw new Error('Invalid response from server - expected JSON but got ' + (contentType || 'unknown content type'));
                }
                
                const result = await response.json();
                
                if (!result.success) {
                    throw new Error(result.error || 'Failed to connect to UniFi Controller');
                }
                
                testBtn.disabled = false;
                testBtn.innerHTML = 'Next<i class="fas fa-arrow-right ml-2"></i>';
                return true;
                
            } catch (error) {
                testBtn.disabled = false;
                testBtn.innerHTML = 'Next<i class="fas fa-arrow-right ml-2"></i>';
                alert('UniFi Connection Failed: ' + error.message);
                return false;
            }
            
        case 3:
            const selectedSite = document.querySelector('input[name="unifi-site"]:checked');
            if (!selectedSite) {
                alert('Please select a site');
                return false;
            }
            return true;
            
        case 4:
            const selectedAP = document.querySelector('input[name="gate-ap"]:checked');
            if (!selectedAP) {
                alert('Please select an access point');
                return false;
            }
            return true;
            
        case 5:
            const shellyUrl = document.getElementById('shelly-url').value;
            const duration = document.getElementById('gate-duration').value;
            
            if (!shellyUrl || !duration) {
                alert('Please fill in all gate configuration fields');
                return false;
            }
            
            try {
                new URL(shellyUrl);
            } catch {
                alert('Please enter a valid Shelly URL');
                return false;
            }
            
            return true;
            
        default:
            return true;
    }
}

function saveStepData(step) {
    switch (step) {
        case 1:
            setupData.admin.username = document.getElementById('admin-username').value;
            setupData.admin.password = document.getElementById('admin-password').value;
            break;
            
        case 2:
            setupData.unifi.controller_url = document.getElementById('unifi-url').value;
            setupData.unifi.username = document.getElementById('unifi-username').value;
            setupData.unifi.password = document.getElementById('unifi-password').value;
            break;
            
        case 3:
            const selectedSite = document.querySelector('input[name="unifi-site"]:checked');
            setupData.unifi.site_id = selectedSite.value;
            break;
            
        case 4:
            const selectedAP = document.querySelector('input[name="gate-ap"]:checked');
            setupData.unifi.gate_ap_mac = selectedAP.value;
            break;
            
        case 5:
            setupData.shelly.trigger_url = document.getElementById('shelly-url').value;
            setupData.gate.open_duration = parseInt(document.getElementById('gate-duration').value);
            break;
    }
}

async function loadSites() {
    const loadingDiv = document.getElementById('site-loading');
    const listDiv = document.getElementById('site-list');
    const errorDiv = document.getElementById('site-error');
    const optionsDiv = document.getElementById('site-options');
    
    loadingDiv.classList.remove('hidden');
    listDiv.classList.add('hidden');
    errorDiv.classList.add('hidden');
    
    // Get saved UniFi credentials
    const unifiData = {
        controller_url: setupData.unifi.controller_url,
        username: setupData.unifi.username,
        password: setupData.unifi.password,
        site_id: 'default' // Get sites from default first
    };
    
    try {
        // Get sites from UniFi controller
        const response = await fetch('/api/test-unifi-sites', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(unifiData)
        });
        
        // Check if we got JSON response
        const contentType = response.headers.get('content-type');
        if (!contentType || !contentType.includes('application/json')) {
            // If not JSON, we probably got redirected or received HTML
            const text = await response.text();
            console.error('Expected JSON but got:', contentType, text.substring(0, 100));
            throw new Error('Invalid response from server - expected JSON but got ' + (contentType || 'unknown content type'));
        }
        
        const result = await response.json();
        
        if (!result.success) {
            throw new Error(result.error || 'Failed to get sites');
        }
        
        availableSites = result.sites || [{ name: 'default', description: 'Default Site' }];
        
        // Display sites
        optionsDiv.innerHTML = '';
        availableSites.forEach(site => {
            const label = document.createElement('label');
            label.className = 'flex items-center p-3 border rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700 border-gray-300 dark:border-gray-600';
            label.innerHTML = `
                <input type="radio" name="unifi-site" value="${site.name}" class="mr-3 text-indigo-600 dark:text-indigo-400" ${site.name === 'default' ? 'checked' : ''}>
                <div>
                    <div class="font-medium text-gray-900 dark:text-white">${site.description || site.name}</div>
                    <div class="text-sm text-gray-500 dark:text-gray-400">Site ID: ${site.name}</div>
                </div>
            `;
            optionsDiv.appendChild(label);
        });
        
        loadingDiv.classList.add('hidden');
        listDiv.classList.remove('hidden');
        
    } catch (error) {
        console.error('Error loading sites:', error);
        loadingDiv.classList.add('hidden');
        errorDiv.classList.remove('hidden');
        
        // If we can't get sites, just use default
        optionsDiv.innerHTML = '';
        const label = document.createElement('label');
        label.className = 'flex items-center p-3 border rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700 border-gray-300 dark:border-gray-600';
        label.innerHTML = `
            <input type="radio" name="unifi-site" value="default" class="mr-3 text-indigo-600 dark:text-indigo-400" checked>
            <div>
                <div class="font-medium text-gray-900 dark:text-white">Default Site</div>
                <div class="text-sm text-gray-500 dark:text-gray-400">Site ID: default</div>
            </div>
        `;
        optionsDiv.appendChild(label);
        
        loadingDiv.classList.add('hidden');
        listDiv.classList.remove('hidden');
    }
}

async function loadAccessPoints() {
    const loadingDiv = document.getElementById('ap-loading');
    const listDiv = document.getElementById('ap-list');
    const errorDiv = document.getElementById('ap-error');
    const optionsDiv = document.getElementById('ap-options');
    
    loadingDiv.classList.remove('hidden');
    listDiv.classList.add('hidden');
    errorDiv.classList.add('hidden');
    
    // Get saved UniFi data with selected site
    const unifiData = {
        controller_url: setupData.unifi.controller_url,
        username: setupData.unifi.username,
        password: setupData.unifi.password,
        site_id: setupData.unifi.site_id
    };
    
    try {
        // Test UniFi connection and get access points
        const response = await fetch('/api/test-unifi', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(unifiData)
        });
        
        // Check if we got JSON response
        const contentType = response.headers.get('content-type');
        if (!contentType || !contentType.includes('application/json')) {
            // If not JSON, we probably got redirected or received HTML
            const text = await response.text();
            console.error('Expected JSON but got:', contentType, text.substring(0, 100));
            throw new Error('Invalid response from server - expected JSON but got ' + (contentType || 'unknown content type'));
        }
        
        const result = await response.json();
        
        if (!result.success) {
            throw new Error(result.error || 'Failed to connect to UniFi');
        }
        
        const aps = result.access_points;
        
        if (!aps || aps.length === 0) {
            throw new Error('No access points found');
        }
        
        // Display access points
        optionsDiv.innerHTML = '';
        aps.forEach(ap => {
            const label = document.createElement('label');
            label.className = 'flex items-center p-3 border rounded-lg cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700 border-gray-300 dark:border-gray-600';
            label.innerHTML = `
                <input type="radio" name="gate-ap" value="${ap.mac}" class="mr-3 text-indigo-600 dark:text-indigo-400">
                <div>
                    <div class="font-medium text-gray-900 dark:text-white">${ap.name || 'Unnamed AP'}</div>
                    <div class="text-sm text-gray-500 dark:text-gray-400">${ap.mac} - ${ap.model || 'Unknown Model'}</div>
                </div>
            `;
            optionsDiv.appendChild(label);
        });
        
        loadingDiv.classList.add('hidden');
        listDiv.classList.remove('hidden');
        
    } catch (error) {
        console.error('Error loading access points:', error);
        loadingDiv.classList.add('hidden');
        errorDiv.classList.remove('hidden');
        
        // Update error message
        const errorMsg = errorDiv.querySelector('p.text-sm');
        if (errorMsg) {
            errorMsg.textContent = error.message || 'Please check your UniFi credentials and try again.';
        }
    }
}

async function finishSetup() {
    if (!validateStep(currentStep)) {
        return;
    }
    
    saveStepData(currentStep);
    
    try {
        const response = await fetch('/api/setup', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(setupData)
        });
        
        if (!response.ok) {
            throw new Error('Setup failed');
        }
        
        // Show success message
        document.getElementById('setup-form').classList.add('hidden');
        document.getElementById('setup-success').classList.remove('hidden');
        
        // Redirect to dashboard after 2 seconds
        setTimeout(() => {
            window.location.href = '/dashboard';
        }, 2000);
        
    } catch (error) {
        console.error('Setup error:', error);
        alert('Setup failed. Please check your settings and try again.');
    }
}

// Initialize
showStep(currentStep);