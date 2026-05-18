// Configuration
const CONFIG = {
    simulatorBaseUrl: 'http://127.0.0.1:9090',
    ingestorBaseUrl: 'http://127.0.0.1:9100',
    bearerToken: 'devtoken',
    devSecret: 'dev',
    autoRefreshInterval: 5000,
};

// State
let autoRefreshTimer = null;
let lastFetchedData = {
    rawGps: [],
    enrichment: [],
    redis: [],
    recommendations: [],
};

// DOM Elements
const stepAllBusesBtn = document.getElementById('stepAllBuses');
const clearLogsBtn = document.getElementById('clearLogs');
const refreshDataBtn = document.getElementById('refreshData');
const autoRefreshCheckbox = document.getElementById('autoRefresh');
const interpolateInput = document.getElementById('interpolatePoints');

const simulatorStatus = document.getElementById('simulatorStatus');
const ingestorStatus = document.getElementById('ingestorStatus');
const redisStatus = document.getElementById('redisStatus');
const lastUpdateSpan = document.getElementById('lastUpdate');

const rawGpsLogs = document.getElementById('rawGpsLogs');
const enrichmentLogs = document.getElementById('enrichmentLogs');
const redisLogs = document.getElementById('redisLogs');
const recommendationsLogs = document.getElementById('recommendationsLogs');
const debugText = document.getElementById('debugText');

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    checkServiceStatus();
    setupEventListeners();
    loadInitialData();
});

function setupEventListeners() {
    stepAllBusesBtn.addEventListener('click', handleStepAllBuses);
    clearLogsBtn.addEventListener('click', clearAllLogs);
    refreshDataBtn.addEventListener('click', loadInitialData);
    autoRefreshCheckbox.addEventListener('change', handleAutoRefreshToggle);
}

// Service Status Check
async function checkServiceStatus() {
    try {
        const simulatorRes = await fetch(`${CONFIG.simulatorBaseUrl}/api/v1/health`, {
            headers: { 
                'Authorization': `Bearer ${CONFIG.bearerToken}`,
            }
        });
        updateStatus(simulatorStatus, simulatorRes.ok);
    } catch (e) {
        updateStatus(simulatorStatus, false);
    }

    try {
        const ingestorRes = await fetch(`${CONFIG.ingestorBaseUrl}/healthz`);
        updateStatus(ingestorStatus, ingestorRes.ok);
    } catch (e) {
        updateStatus(ingestorStatus, false);
    }

    try {
        const redisRes = await fetch(`${CONFIG.simulatorBaseUrl}/api/v1/redis/status`, {
            headers: { 
                'Authorization': `Bearer ${CONFIG.bearerToken}`,
            }
        });
        updateStatus(redisStatus, redisRes.ok);
    } catch (e) {
        updateStatus(redisStatus, false);
    }
}

function updateStatus(element, isHealthy) {
    if (isHealthy) {
        element.textContent = '🟢 Healthy';
        element.style.color = 'var(--color-success)';
    } else {
        element.textContent = '🔴 Down';
        element.style.color = 'var(--color-error)';
    }
}

// Step All Buses
async function handleStepAllBuses() {
    const interpolate = interpolateInput.value;
    stepAllBusesBtn.disabled = true;
    stepAllBusesBtn.innerHTML = '⏳ Running... <span class="loading"></span>';

    try {
        const response = await fetch(
            `${CONFIG.simulatorBaseUrl}/api/v1/buses/simulate-all-gps?interpolate=${interpolate}`,
            {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${CONFIG.bearerToken}`,
                    'X-Dev-Secret': CONFIG.devSecret,
                    'Content-Type': 'application/json',
                }
            }
        );

        if (!response.ok) {
            throw new Error(`Simulation failed: ${response.statusText}`);
        }

        const result = await response.json();
        updateDebugInfo('GPS Simulation', result);

        // Wait for data to propagate through pipeline
        await new Promise(resolve => setTimeout(resolve, 1000));

        // Fetch updated data
        await loadInitialData();

        showNotification('✅ GPS simulation completed successfully', 'success');
    } catch (error) {
        console.error('Error stepping buses:', error);
        showNotification(`❌ Error: ${error.message}`, 'error');
        updateDebugInfo('Error', { error: error.message, stack: error.stack });
    } finally {
        stepAllBusesBtn.disabled = false;
        stepAllBusesBtn.innerHTML = '▶️ Step All Buses';
    }
}

// Load Initial Data
async function loadInitialData() {
    refreshDataBtn.disabled = true;
    refreshDataBtn.innerHTML = '⏳ Loading...';

    try {
        // Fetch all buses first to get bus IDs
        const buses = await fetchBuses();
        const busIds = buses.map(b => b.busId || b.bus_id).filter(Boolean);

        if (busIds.length === 0) {
            showNotification('⚠️ No buses found. Load sample data first.', 'warning');
            return;
        }

        // Fetch journeys to get journey IDs
        const journeys = await fetchJourneys();
        const journeyIds = journeys.map(j => j.journeyId || j.journey_id).filter(Boolean);

        // Fetch data from all stages
        const [rawGpsData, redisData, recommendationData] = await Promise.all([
            fetchIngestorData(busIds),
            fetchRedisData(busIds),
            fetchRecommendationData(journeyIds),
        ]);

        lastFetchedData.rawGps = rawGpsData;
        lastFetchedData.redis = redisData;
        lastFetchedData.recommendations = recommendationData;

        // Render all stages
        renderRawGps(rawGpsData);
        renderEnrichment(rawGpsData); // Enrichment comes from ingestor (what it computed)
        renderRedis(redisData); // Redis is final stored state
        renderRecommendations(recommendationData);

        lastUpdateSpan.textContent = new Date().toLocaleTimeString();
        showNotification('✅ Data refreshed', 'success');
    } catch (error) {
        console.error('Error loading data:', error);
        showNotification(`❌ Error loading data: ${error.message}`, 'error');
    } finally {
        refreshDataBtn.disabled = false;
        refreshDataBtn.innerHTML = '🔄 Refresh Data';
    }
}

// Fetch Functions
async function fetchBuses() {
    const response = await fetch(`${CONFIG.simulatorBaseUrl}/api/v1/buses`, {
        headers: { 
            'Authorization': `Bearer ${CONFIG.bearerToken}`,
        }
    });
    if (!response.ok) throw new Error('Failed to fetch buses');
    return await response.json();
}

async function fetchJourneys() {
    const response = await fetch(`${CONFIG.simulatorBaseUrl}/api/v1/journeys`, {
        headers: { 
            'Authorization': `Bearer ${CONFIG.bearerToken}`,
        }
    });
    if (!response.ok) throw new Error('Failed to fetch journeys');
    return await response.json();
}

async function fetchIngestorData(busIds) {
    const results = await Promise.all(
        busIds.map(async (busId) => {
            try {
                const response = await fetch(`${CONFIG.ingestorBaseUrl}/dev/obs/bus?busId=${busId}`);
                if (!response.ok) return null;
                const data = await response.json();
                return data.found ? { busId, ...data.event } : null;
            } catch (e) {
                return null;
            }
        })
    );
    return results.filter(Boolean);
}

async function fetchRedisData(busIds) {
    const results = await Promise.all(
        busIds.map(async (busId) => {
            try {
                const response = await fetch(
                    `${CONFIG.simulatorBaseUrl}/api/v1/dev/obs/redis/bus?busId=${busId}`,
                    {
                        headers: {
                            'Authorization': `Bearer ${CONFIG.bearerToken}`,
                            'X-Dev-Secret': CONFIG.devSecret,
                        }
                    }
                );
                if (!response.ok) return null;
                const data = await response.json();
                return data.exists ? { busId, ...data } : null;
            } catch (e) {
                return null;
            }
        })
    );
    return results.filter(Boolean);
}

async function fetchRecommendationData(journeyIds) {
    const results = await Promise.all(
        journeyIds.slice(0, 10).map(async (journeyId) => { // Limit to first 10 journeys
            try {
                const response = await fetch(
                    `${CONFIG.simulatorBaseUrl}/api/v1/dev/obs/journey-explain?journeyId=${journeyId}`,
                    {
                        headers: {
                            'Authorization': `Bearer ${CONFIG.bearerToken}`,
                            'X-Dev-Secret': CONFIG.devSecret,
                        }
                    }
                );
                if (!response.ok) return null;
                return await response.json();
            } catch (e) {
                return null;
            }
        })
    );
    return results.filter(Boolean);
}

// Render Functions
function renderRawGps(data) {
    if (!data || data.length === 0) {
        rawGpsLogs.innerHTML = '<div class="empty-state">No raw GPS data available</div>';
        return;
    }

    rawGpsLogs.innerHTML = data.map(entry => `
        <div class="log-entry">
            <div class="log-header">
                <span class="log-bus-id">${entry.busId || entry.bus_id}</span>
                <span class="log-timestamp">${formatTimestamp(entry.received_at)}</span>
            </div>
            <div class="log-content">
                <div class="log-field">
                    <span class="log-field-label">📍 Lat/Lon:</span>
                    <span class="log-field-value">${entry.raw_lat?.toFixed(4)}, ${entry.raw_lon?.toFixed(4)}</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">🚄 Speed:</span>
                    <span class="log-field-value">${entry.raw_speed} km/h</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">⏰ Device Time:</span>
                    <span class="log-field-value">${new Date(entry.raw_device_ts_ms).toLocaleTimeString()}</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">📨 MQTT Topic:</span>
                    <span class="log-field-value">${entry.topic}</span>
                </div>
            </div>
        </div>
    `).join('');
}

function renderEnrichment(data) {
    if (!data || data.length === 0) {
        enrichmentLogs.innerHTML = '<div class="empty-state">No enrichment data available</div>';
        return;
    }

    enrichmentLogs.innerHTML = data.map(entry => {
        const enriched = entry.enriched || false;
        const rejected = entry.rejection_reason;
        
        return `
        <div class="log-entry">
            <div class="log-header">
                <span class="log-bus-id">${entry.busId || entry.bus_id}</span>
                <span class="badge ${enriched ? 'badge-success' : 'badge-error'}">
                    ${enriched ? 'Enriched' : 'Not Enriched'}
                </span>
            </div>
            <div class="log-content">
                ${enriched ? `
                    <div class="log-field">
                        <span class="log-field-label">🚏 Route:</span>
                        <span class="log-field-value">${entry.route_id || 'N/A'}</span>
                    </div>
                    <div class="log-field">
                        <span class="log-field-label">🧭 Direction:</span>
                        <span class="log-field-value">${entry.direction === 0 ? 'North' : entry.direction === 1 ? 'South' : 'N/A'}</span>
                    </div>
                    <div class="log-field">
                        <span class="log-field-label">📍 Stop:</span>
                        <span class="log-field-value">${entry.stop_id || 'N/A'} (Index: ${entry.stop_index ?? 'N/A'})</span>
                    </div>
                    <div class="log-field">
                        <span class="log-field-label">🎯 Confidence:</span>
                        <span class="log-field-value">${entry.confidence?.toFixed(2) || 'N/A'}</span>
                    </div>
                    <div class="log-field">
                        <span class="log-field-label">🔄 Interpolated:</span>
                        <span class="log-field-value">${entry.interpolated ? '✅ Yes' : '❌ No'}</span>
                    </div>
                    <div class="log-field">
                        <span class="log-field-label">🏁 At Terminal:</span>
                        <span class="log-field-value">${entry.is_at_terminal ? '✅ Yes' : '❌ No'}</span>
                    </div>
                    <div class="log-field">
                        <span class="log-field-label">📏 Distance:</span>
                        <span class="log-field-value">${entry.distance_from_stop_m?.toFixed(1) || 0} m</span>
                    </div>
                ` : rejected ? `
                    <div class="log-field">
                        <span class="log-field-label">❌ Rejected:</span>
                        <span class="log-field-value badge badge-error">${rejected}</span>
                    </div>
                    <div class="log-field">
                        <span class="log-field-label">ℹ️ Explanation:</span>
                        <span class="log-field-value" style="font-size: 0.85em;">${explainRejectionReason(rejected)}</span>
                    </div>
                ` : `
                    <div class="log-field">
                        <span class="log-field-label">⚠️ Status:</span>
                        <span class="log-field-value badge badge-warning">Waiting for enrichment</span>
                    </div>
                `}
            </div>
        </div>
    `;
    }).join('');
}

function explainRejectionReason(reason) {
    const explanations = {
        'impossible_gps_jump': 'Bus teleported too far instantly (data quality protection)',
        'update is not newer than current state': 'Duplicate or old GPS timestamp',
        'device_ts_far_future': 'Device timestamp is too far in the future',
        'device_ts_far_past(replay)': 'Device timestamp is too old (replay protection)',
        'validation_failed': 'GPS coordinates failed validation',
        'parse_error': 'Could not parse GPS payload',
    };
    return explanations[reason] || reason;
}

function renderRedis(data) {
    if (!data || data.length === 0) {
        redisLogs.innerHTML = '<div class="empty-state">No Redis data available</div>';
        return;
    }

    redisLogs.innerHTML = data.map(entry => `
        <div class="log-entry">
            <div class="log-header">
                <span class="log-bus-id">${entry.busId || entry.bus_id}</span>
                <span class="badge badge-info">TTL: ${entry.ttl_seconds}s</span>
            </div>
            <div class="log-content">
                <div class="log-field">
                    <span class="log-field-label">🚏 Route:</span>
                    <span class="log-field-value">${entry.parsed?.route_id || 'N/A'}</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">🧭 Direction:</span>
                    <span class="log-field-value">${entry.parsed?.direction === 0 ? 'North' : 'South'}</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">📍 Stop:</span>
                    <span class="log-field-value">${entry.parsed?.stop_id || 'N/A'} (Index: ${entry.parsed?.stop_index})</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">📍 Lat/Lon:</span>
                    <span class="log-field-value">${entry.parsed?.lat?.toFixed(4)}, ${entry.parsed?.lon?.toFixed(4)}</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">🚄 Speed:</span>
                    <span class="log-field-value">${entry.parsed?.speed_kmh} km/h</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">🔑 Redis Key:</span>
                    <span class="log-field-value" style="font-size: 0.75rem;">${entry.redis_key}</span>
                </div>
            </div>
        </div>
    `).join('');
}

function renderRecommendations(data) {
    if (!data || data.length === 0) {
        recommendationsLogs.innerHTML = '<div class="empty-state">No journey recommendations available</div>';
        return;
    }

    recommendationsLogs.innerHTML = data.map(journey => `
        <div class="log-entry">
            <div class="log-header">
                <span class="log-bus-id">Journey: ${journey.journey_id?.substring(0, 8)}...</span>
                <span class="badge badge-${getStatusColor(journey.status_name)}">${journey.status_name}</span>
            </div>
            <div class="log-content">
                <div class="log-field">
                    <span class="log-field-label">👤 User:</span>
                    <span class="log-field-value">${journey.user_id}</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">🚏 Route:</span>
                    <span class="log-field-value">${journey.origin_stop_id} → ${journey.destination_stop_id}</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">🚌 Active Bus:</span>
                    <span class="log-field-value">${journey.active_bus_id || 'None'}</span>
                </div>
                <div class="log-field">
                    <span class="log-field-label">📊 Recommendations:</span>
                    <span class="log-field-value">${journey.recommendations_count} buses</span>
                </div>
            </div>
            ${journey.recommendations && journey.recommendations.length > 0 ? `
                <div style="margin-top: 10px; padding-top: 10px; border-top: 1px solid var(--border-color);">
                    ${journey.recommendations.slice(0, 3).map((rec, idx) => `
                        <div class="recommendation-card">
                            <span class="recommendation-rank">${idx + 1}</span>
                            <strong>${rec.busId}</strong>
                            <div style="margin-top: 5px; font-size: 0.85rem; color: var(--text-secondary);">
                                ⏱️ ETA: ${formatDuration(rec.estimatedArrival)} | 
                                📏 ${(rec.distanceMeters / 1000).toFixed(2)} km | 
                                🎯 ${(rec.confidenceLevel * 100).toFixed(0)}%
                            </div>
                            <div style="margin-top: 3px; font-size: 0.8rem; font-style: italic;">
                                ${rec.displayText}
                            </div>
                        </div>
                    `).join('')}
                </div>
            ` : ''}
        </div>
    `).join('');
}

// Helper Functions
function formatTimestamp(timestamp) {
    if (!timestamp) return 'N/A';
    try {
        return new Date(timestamp).toLocaleTimeString();
    } catch {
        return 'Invalid';
    }
}

function formatDuration(ms) {
    if (!ms) return '0s';
    const minutes = Math.floor(ms / 60000);
    const seconds = Math.floor((ms % 60000) / 1000);
    return minutes > 0 ? `${minutes}m ${seconds}s` : `${seconds}s`;
}

function getStatusColor(status) {
    const statusMap = {
        'Tracking': 'success',
        'Completed': 'info',
        'Cancelled': 'error',
        'Boarding': 'warning',
    };
    return statusMap[status] || 'info';
}

function updateDebugInfo(title, data) {
    debugText.textContent = `=== ${title} ===\n${JSON.stringify(data, null, 2)}`;
}

function showNotification(message, type) {
    // Simple console notification for now
    console.log(`[${type.toUpperCase()}] ${message}`);
    // Could be enhanced with a toast notification system
}

function clearAllLogs() {
    rawGpsLogs.innerHTML = '<div class="empty-state">Logs cleared</div>';
    enrichmentLogs.innerHTML = '<div class="empty-state">Logs cleared</div>';
    redisLogs.innerHTML = '<div class="empty-state">Logs cleared</div>';
    recommendationsLogs.innerHTML = '<div class="empty-state">Logs cleared</div>';
    debugText.textContent = 'No debug data yet';
    lastFetchedData = { rawGps: [], enrichment: [], redis: [], recommendations: [] };
}

function handleAutoRefreshToggle(e) {
    if (e.target.checked) {
        autoRefreshTimer = setInterval(loadInitialData, CONFIG.autoRefreshInterval);
        showNotification('✅ Auto-refresh enabled', 'success');
    } else {
        if (autoRefreshTimer) {
            clearInterval(autoRefreshTimer);
            autoRefreshTimer = null;
        }
        showNotification('⏸️ Auto-refresh disabled', 'info');
    }
}
