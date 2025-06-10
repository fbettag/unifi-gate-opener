// Tab management
function showTab(tabName) {
    // Hide all tabs
    document.querySelectorAll('.tab-content').forEach(tab => {
        tab.classList.remove('active');
    });
    
    // Remove active class from all buttons
    document.querySelectorAll('.tab-button').forEach(btn => {
        btn.classList.remove('border-indigo-500', 'text-indigo-600', 'active');
        btn.classList.add('border-transparent');
        // Ensure color stays light
        btn.style.setProperty('color', '#d1d5db', 'important');
    });
    
    // Show selected tab
    document.getElementById(tabName + '-tab').classList.add('active');
    
    // Mark button as active
    const activeButton = event.target.closest('.tab-button');
    activeButton.classList.remove('border-transparent');
    activeButton.classList.add('border-indigo-500', 'active');
    // Active button color
    activeButton.style.setProperty('color', '#818cf8', 'important');
    
    // Load tab-specific data
    switch (tabName) {
        case 'devices':
            loadDevices();
            break;
        case 'logs':
            loadLogs();
            break;
        case 'settings':
            loadSettings();
            break;
    }
}

// Device management
async function loadDevices() {
    try {
        const response = await fetch('/api/devices');
        const devices = await response.json();
        console.log('Loaded devices:', devices);
        const statusResponse = await fetch('/api/status');
        const status = await statusResponse.json();
        
        const tbody = document.getElementById('devices-list');
        tbody.innerHTML = '';
        
        devices.forEach(device => {
            const deviceStatus = status.devices[device.mac] || {};
            const row = document.createElement('tr');
            row.innerHTML = `
                <td class="px-6 py-4 whitespace-nowrap">
                    <div class="text-sm font-medium text-gray-900 dark:text-white">${device.name}</div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                    <div class="text-sm text-gray-500 dark:text-gray-400">${device.mac}</div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                    <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                        deviceStatus.is_connected 
                            ? 'bg-green-100 text-green-800' 
                            : 'bg-gray-100 text-gray-800'
                    }">
                        ${deviceStatus.is_connected ? 'Connected' : 'Disconnected'}
                    </span>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    ${deviceStatus.last_seen ? new Date(deviceStatus.last_seen).toLocaleString() : 'Never'}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    <button onclick="deleteDevice('${device.mac}')" class="text-red-600 hover:text-red-900">
                        <i class="fas fa-trash"></i>
                    </button>
                </td>
            `;
            tbody.appendChild(row);
        });
    } catch (error) {
        console.error('Error loading devices:', error);
    }
}

let unifiClients = [];

async function showAddDevice() {
    document.getElementById('add-device-modal').classList.remove('hidden');
    
    // Load UniFi clients
    try {
        console.log('Loading UniFi clients...');
        const response = await fetch('/api/unifi/clients', {
            credentials: 'same-origin'
        });
        console.log('Response status:', response.status);
        if (response.ok) {
            unifiClients = await response.json();
            console.log('Loaded clients:', unifiClients.length);
            renderClientList(unifiClients);
        } else {
            console.error('Failed response:', response.status, await response.text());
        }
    } catch (error) {
        console.error('Failed to load UniFi clients:', error);
    }
}

function hideAddDevice() {
    document.getElementById('add-device-modal').classList.add('hidden');
    document.getElementById('new-device-name').value = '';
    document.getElementById('new-device-mac').value = '';
    document.getElementById('client-search').value = '';
    hideClientDropdown();
}

function showClientDropdown() {
    console.log('Showing client dropdown');
    document.getElementById('client-dropdown').classList.remove('hidden');
}

function hideClientDropdown() {
    console.log('Hiding client dropdown');
    document.getElementById('client-dropdown').classList.add('hidden');
}

function filterClients() {
    const searchTerm = document.getElementById('client-search').value.toLowerCase();
    const filteredClients = unifiClients.filter(client => {
        const name = (client.name || client.hostname || client.mac).toLowerCase();
        return name.includes(searchTerm);
    });
    renderClientList(filteredClients);
}

function renderClientList(clients) {
    console.log('Rendering client list with', clients.length, 'clients');
    const clientList = document.getElementById('client-list');
    clientList.innerHTML = '';
    
    clients.forEach(client => {
        const li = document.createElement('li');
        li.className = 'px-4 py-2 hover:bg-gray-100 dark:hover:bg-gray-600 cursor-pointer';
        const displayName = client.name || client.hostname || 'Unknown Device';
        li.innerHTML = `
            <div class="flex justify-between items-center">
                <div>
                    <div class="text-sm font-medium text-gray-900 dark:text-white">${displayName}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">${client.mac}</div>
                </div>
                ${client.ip ? `<div class="text-xs text-gray-400 dark:text-gray-300">${client.ip}</div>` : ''}
            </div>
        `;
        li.onclick = () => selectClient(client);
        clientList.appendChild(li);
    });
    
    if (clients.length === 0) {
        const li = document.createElement('li');
        li.className = 'px-4 py-2 text-sm text-gray-500';
        li.textContent = 'No clients found';
        clientList.appendChild(li);
    }
}

function selectClient(client) {
    const displayName = client.name || client.hostname || 'Unknown Device';
    document.getElementById('new-device-name').value = displayName;
    document.getElementById('new-device-mac').value = client.mac;
    document.getElementById('client-search').value = `${displayName} (${client.mac})`;
    hideClientDropdown();
}

async function addDevice() {
    const name = document.getElementById('new-device-name').value;
    const mac = document.getElementById('new-device-mac').value.toUpperCase();
    
    if (!name || !mac) {
        alert('Please fill in all fields');
        return;
    }
    
    // Basic MAC address validation
    const macRegex = /^([0-9A-F]{2}[:-]){5}([0-9A-F]{2})$/;
    if (!macRegex.test(mac)) {
        alert('Please enter a valid MAC address');
        return;
    }
    
    try {
        const response = await fetch('/api/devices', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ name, mac })
        });
        
        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }
        
        hideAddDevice();
        loadDevices();
    } catch (error) {
        alert('Failed to add device: ' + error.message);
    }
}

async function deleteDevice(mac) {
    if (!confirm('Are you sure you want to delete this device?')) {
        return;
    }
    
    try {
        const response = await fetch(`/api/devices/${encodeURIComponent(mac)}`, {
            method: 'DELETE',
            credentials: 'same-origin'
        });
        
        if (!response.ok) {
            throw new Error('Failed to delete device');
        }
        
        loadDevices();
    } catch (error) {
        alert('Failed to delete device: ' + error.message);
    }
}

// Logs
async function loadLogs() {
    try {
        const response = await fetch('/api/logs?limit=100');
        const logs = await response.json();
        
        const tbody = document.getElementById('logs-list');
        tbody.innerHTML = '';
        
        if (logs && Array.isArray(logs)) {
            logs.forEach(log => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    ${new Date(log.timestamp).toLocaleString()}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                    ${log.device_name} (${log.device_mac})
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                    <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                        log.event === 'gate_triggered' ? 'bg-green-100 text-green-800' :
                        log.event === 'connected' ? 'bg-blue-100 text-blue-800' :
                        log.event === 'disconnected' ? 'bg-gray-100 text-gray-800' :
                        log.event === 'roamed' ? 'bg-yellow-100 text-yellow-800' :
                        'bg-red-100 text-red-800'
                    }">
                        ${log.event.replace('_', ' ')}
                    </span>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    ${log.direction || '-'}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                    ${log.gate_opened ? '<i class="fas fa-check text-green-500"></i>' : '-'}
                </td>
                <td class="px-6 py-4 text-sm text-gray-500 dark:text-gray-400">
                    ${log.message}
                </td>
            `;
            tbody.appendChild(row);
        });
        }
    } catch (error) {
        console.error('Error loading logs:', error);
    }
}

// Settings
async function loadSettings() {
    try {
        const response = await fetch('/api/settings');
        const settings = await response.json();
        
        // UniFi settings
        document.getElementById('settings-unifi-url').value = settings.unifi.controller_url;
        document.getElementById('settings-unifi-username').value = settings.unifi.username;
        document.getElementById('settings-unifi-site').value = settings.unifi.site_id;
        document.getElementById('settings-poll-interval').value = settings.unifi.poll_interval;
        
        // Gate settings
        document.getElementById('settings-shelly-url').value = settings.shelly.trigger_url;
        document.getElementById('settings-open-duration').value = settings.gate.open_duration;
        document.getElementById('settings-log-activity').checked = settings.gate.log_activity || false;
        
        // Load access points
        const apsResponse = await fetch('/api/unifi/aps');
        if (apsResponse.ok) {
            const aps = await apsResponse.json();
            const select = document.getElementById('settings-gate-ap');
            select.innerHTML = '';
            
            aps.forEach(ap => {
                const option = document.createElement('option');
                option.value = ap.mac;
                option.textContent = `${ap.name || 'Unnamed'} (${ap.mac})`;
                if (ap.mac === settings.unifi.gate_ap_mac) {
                    option.selected = true;
                }
                select.appendChild(option);
            });
        }
    } catch (error) {
        console.error('Error loading settings:', error);
    }
}

async function saveSettings() {
    // Get settings
    const settings = {
        unifi: {
            controller_url: document.getElementById('settings-unifi-url').value,
            username: document.getElementById('settings-unifi-username').value,
            password: document.getElementById('settings-unifi-password').value,
            site_id: document.getElementById('settings-unifi-site').value,
            gate_ap_mac: document.getElementById('settings-gate-ap').value,
            poll_interval: parseInt(document.getElementById('settings-poll-interval').value)
        },
        shelly: {
            trigger_url: document.getElementById('settings-shelly-url').value
        },
        gate: {
            open_duration: parseInt(document.getElementById('settings-open-duration').value),
            log_activity: document.getElementById('settings-log-activity').checked
        }
    };
    
    // If UniFi settings changed and password provided, test connection first
    if (settings.unifi.password) {
        const testBtn = event.target;
        testBtn.disabled = true;
        testBtn.innerHTML = '<i class="fas fa-spinner fa-spin mr-2"></i>Testing UniFi...';
        
        try {
            const testResponse = await fetch('/api/test-unifi', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    controller_url: settings.unifi.controller_url,
                    username: settings.unifi.username,
                    password: settings.unifi.password,
                    site_id: settings.unifi.site_id
                })
            });
            
            const testResult = await testResponse.json();
            
            if (!testResult.success) {
                throw new Error(testResult.error || 'Failed to connect to UniFi');
            }
        } catch (error) {
            testBtn.disabled = false;
            testBtn.innerHTML = '<i class="fas fa-save mr-2"></i>Save Settings';
            alert('UniFi connection test failed: ' + error.message);
            return;
        }
    }
    
    try {
        const response = await fetch('/api/settings', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(settings)
        });
        
        if (!response.ok) {
            throw new Error('Failed to save settings');
        }
        
        if (event.target) {
            event.target.disabled = false;
            event.target.innerHTML = '<i class="fas fa-save mr-2"></i>Save Settings';
        }
        
        alert('Settings saved successfully!');
        
        // Reload access points if UniFi settings changed
        if (settings.unifi.password) {
            loadSettings();
        }
    } catch (error) {
        if (event.target) {
            event.target.disabled = false;
            event.target.innerHTML = '<i class="fas fa-save mr-2"></i>Save Settings';
        }
        alert('Failed to save settings: ' + error.message);
    }
}

// Gate control
async function testGate() {
    if (!confirm('Are you sure you want to open the gate?')) {
        return;
    }
    
    try {
        const response = await fetch('/api/test-gate', {
            method: 'POST'
        });
        
        if (!response.ok) {
            throw new Error('Failed to trigger gate');
        }
        
        alert('Gate opened successfully!');
        // Reload activity
        updateRecentActivity();
    } catch (error) {
        alert('Failed to open gate: ' + error.message);
    }
}

// Real-time updates
async function updateStatus() {
    try {
        const response = await fetch('/api/status');
        const status = await response.json();
        
        // Update connected count
        const connectedCount = Object.values(status.devices).filter(d => d.is_connected).length;
        document.getElementById('connected-count').textContent = connectedCount;
        
    } catch (error) {
        console.error('Error updating status:', error);
    }
}

async function updateRecentActivity() {
    try {
        const response = await fetch('/api/logs?limit=10');
        const logs = await response.json();
        
        const ul = document.getElementById('recent-activity');
        ul.innerHTML = '';
        
        if (logs && Array.isArray(logs)) {
            logs.forEach(log => {
            const li = document.createElement('li');
            li.className = 'px-4 py-4 sm:px-6';
            
            let icon = '';
            let iconColor = '';
            
            switch (log.event) {
                case 'gate_triggered':
                    icon = 'fa-door-open';
                    iconColor = 'text-green-500';
                    break;
                case 'connected':
                    icon = 'fa-wifi';
                    iconColor = 'text-blue-500';
                    break;
                case 'disconnected':
                    icon = 'fa-wifi';
                    iconColor = 'text-gray-400';
                    break;
                case 'roamed':
                    icon = 'fa-exchange-alt';
                    iconColor = 'text-yellow-500';
                    break;
                default:
                    icon = 'fa-info-circle';
                    iconColor = 'text-gray-500';
            }
            
            li.innerHTML = `
                <div class="flex items-center justify-between">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <i class="fas ${icon} ${iconColor}"></i>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-900 dark:text-white">
                                ${log.device_name} ${log.device_mac ? `(${log.device_mac})` : ''}
                            </p>
                            <p class="text-sm text-gray-500 dark:text-gray-400">
                                ${log.message}
                            </p>
                        </div>
                    </div>
                    <div class="text-sm text-gray-500 dark:text-gray-400">
                        ${new Date(log.timestamp).toLocaleTimeString()}
                    </div>
                </div>
            `;
            
            ul.appendChild(li);
        });
        }
    } catch (error) {
        console.error('Error updating activity:', error);
    }
}

// Initialize and set up auto-refresh
document.addEventListener('DOMContentLoaded', () => {
    updateStatus();
    updateRecentActivity();
    
    // Update every 10 seconds
    setInterval(() => {
        updateStatus();
        updateRecentActivity();
    }, 10000);
});