/*
  admin_ui
  Vanilla JS (no build step).
  Calls Simulation Preview endpoints:
    - POST /api/v1/buses/device/provision
    - POST /api/v1/buses/device/revoke
*/

function $(id) {
  return document.getElementById(id);
}

function normalizeBaseUrl(url) {
  const trimmed = (url || "").trim().replace(/\/+$/, "");
  return trimmed || "http://localhost:8081";
}

function setStatus(el, kind, msg) {
  el.classList.remove("ok", "err", "warn");
  if (kind) el.classList.add(kind);
  el.textContent = msg || "";
}

async function apiFetch(baseUrl, path, init) {
  const res = await fetch(`${baseUrl}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...(init && init.headers ? init.headers : {}),
    },
    ...init,
  });

  const text = await res.text();
  let json;
  try {
    json = text ? JSON.parse(text) : null;
  } catch {
    json = null;
  }

  if (!res.ok) {
    const errMsg = (json && (json.error || json.message)) || text || `HTTP ${res.status}`;
    const err = new Error(errMsg);
    err.status = res.status;
    err.body = json;
    throw err;
  }

  return json;
}

function payloadTemplate(busId) {
  const now = Date.now();
  return JSON.stringify(
    {
      bus_id: busId,
      lat: -20.312,
      lon: 57.367,
      timestamp_ms: now,
      speed_kmh: 22.4,
      heading: 180,
      accuracy_m: 6.0,
    },
    null,
    2
  );
}

function downloadJson(filename, data) {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

function maskPassword(value) {
  if (!value) return "";
  return "•".repeat(Math.min(value.length, 20));
}

(function main() {
  const baseUrlEl = $("baseUrl");
  const busIdEl = $("busId");
  const rotateEl = $("rotate");

  const healthStatusEl = $("healthStatus");
  const opStatusEl = $("opStatus");

  const resultEl = $("result");
  const outTopicEl = $("outTopic");
  const outUserEl = $("outUser");
  const outPassEl = $("outPass");
  const outDeviceEl = $("outDevice");
  const payloadEl = $("payloadTemplate");

  const btnHealth = $("btnHealth");
  const btnOpenSwagger = $("btnOpenSwagger");
  const btnProvision = $("btnProvision");
  const btnRevoke = $("btnRevoke");
  const btnDownload = $("btnDownload");
  const btnHidePassword = $("btnHidePassword");

  const gpsLatEl = $("gpsLat");
  const gpsLonEl = $("gpsLon");
  const gpsSpeedEl = $("gpsSpeed");
  const gpsHeadingEl = $("gpsHeading");
  const btnSendGps = $("btnSendGps");
  const gpsStatusEl = $("gpsStatus");

  const STORAGE_KEY = "alebus.admin_ui.v1";

  function loadState() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return;
      const st = JSON.parse(raw);
      if (st.baseUrl) baseUrlEl.value = st.baseUrl;
      if (st.busId) busIdEl.value = st.busId;
      if (typeof st.rotate === "boolean") rotateEl.checked = st.rotate;
    } catch {
      // ignore
    }
  }

  function saveState() {
    try {
      const st = {
        baseUrl: normalizeBaseUrl(baseUrlEl.value),
        busId: (busIdEl.value || "").trim(),
        rotate: !!rotateEl.checked,
      };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(st));
    } catch {
      // ignore
    }
  }

  function showResult(data) {
    resultEl.hidden = false;
    outTopicEl.textContent = data.topic || "";
    outUserEl.textContent = data.mqttUsername || "";
    outPassEl.textContent = data.mqttPassword || "";
    outDeviceEl.textContent = data.deviceId || "";

    payloadEl.textContent = payloadTemplate((busIdEl.value || "").trim());

    // Don't persist the password.
    saveState();
  }

  function clearResult() {
    resultEl.hidden = true;
    outTopicEl.textContent = "";
    outUserEl.textContent = "";
    outPassEl.textContent = "";
    outDeviceEl.textContent = "";
  }

  baseUrlEl.addEventListener("change", saveState);
  busIdEl.addEventListener("change", saveState);
  rotateEl.addEventListener("change", saveState);

  btnOpenSwagger.addEventListener("click", () => {
    const baseUrl = normalizeBaseUrl(baseUrlEl.value);
    window.open(`${baseUrl}/api/openapi.yaml`, "_blank");
  });

  btnHealth.addEventListener("click", async () => {
    setStatus(healthStatusEl, null, "Checking...");
    const baseUrl = normalizeBaseUrl(baseUrlEl.value);
    saveState();

    try {
      // Try a cheap endpoint first.
      await apiFetch(baseUrl, "/api/v1/redis/status", { method: "GET" });
      setStatus(healthStatusEl, "ok", `OK: ${baseUrl} reachable`);
    } catch (e) {
      setStatus(
        healthStatusEl,
        "err",
        `Failed: ${baseUrl} (${e.message || "unknown error"})`
      );
    }
  });

  async function provision() {
    const baseUrl = normalizeBaseUrl(baseUrlEl.value);
    const busId = (busIdEl.value || "").trim();
    const rotateIfExists = !!rotateEl.checked;

    clearResult();
    setStatus(opStatusEl, null, "Provisioning...");

    if (!busId) {
      setStatus(opStatusEl, "warn", "Bus ID is required");
      return;
    }

    try {
      const data = await apiFetch(baseUrl, "/api/v1/buses/device/provision", {
        method: "POST",
        body: JSON.stringify({ busId, rotateIfExists }),
      });
      showResult(data);
      setStatus(opStatusEl, "ok", "Provisioned. Copy creds into the Android app/device now.");
    } catch (e) {
      const hint = e.status === 404 ? " (bus not found)" : e.status === 409 ? " (active device exists)" : "";
      setStatus(opStatusEl, "err", `Provision failed${hint}: ${e.message || "unknown error"}`);
    }
  }

  async function revoke() {
    const baseUrl = normalizeBaseUrl(baseUrlEl.value);
    const busId = (busIdEl.value || "").trim();

    clearResult();
    setStatus(opStatusEl, null, "Revoking...");

    if (!busId) {
      setStatus(opStatusEl, "warn", "Bus ID is required");
      return;
    }

    try {
      await apiFetch(baseUrl, "/api/v1/buses/device/revoke", {
        method: "POST",
        body: JSON.stringify({ busId }),
      });
      setStatus(opStatusEl, "ok", "Revoked active device.");
    } catch (e) {
      const hint = e.status === 404 ? " (no active device)" : "";
      setStatus(opStatusEl, "err", `Revoke failed${hint}: ${e.message || "unknown error"}`);
    }
  }

  btnProvision.addEventListener("click", () => provision());
  btnRevoke.addEventListener("click", () => revoke());

  async function sendTestGPS() {
    const baseUrl = normalizeBaseUrl(baseUrlEl.value);
    const busId = (busIdEl.value || "").trim();

    setStatus(gpsStatusEl, null, "Publishing...");
    if (!busId) {
      setStatus(gpsStatusEl, "warn", "Bus ID is required");
      return;
    }

    const lat = parseFloat(gpsLatEl.value);
    const lon = parseFloat(gpsLonEl.value);
    const speedKmh = parseFloat(gpsSpeedEl.value || "0") || 0;
    const heading = parseFloat(gpsHeadingEl.value || "0") || 0;

    if (!Number.isFinite(lat) || !Number.isFinite(lon)) {
      setStatus(gpsStatusEl, "warn", "Latitude/Longitude must be valid numbers");
      return;
    }

    try {
      const data = await apiFetch(baseUrl, "/api/v1/buses/simulate-gps", {
        method: "POST",
        body: JSON.stringify({
          bus_id: busId,
          lat,
          lon,
          speed_kmh: speedKmh,
          heading,
        }),
      });
      const topic = (data && data.topic) ? ` (${data.topic})` : "";
      setStatus(gpsStatusEl, "ok", `Published raw GPS for ${busId}${topic}`);
    } catch (e) {
      setStatus(gpsStatusEl, "err", `Publish failed: ${e.message || "unknown error"}`);
    }
  }

  if (btnSendGps) {
    btnSendGps.addEventListener("click", () => sendTestGPS());
  }

  document.body.addEventListener("click", async (evt) => {
    const btn = evt.target.closest("button[data-copy]");
    if (!btn) return;
    const id = btn.getAttribute("data-copy");
    const el = $(id);
    const text = (el && el.textContent) || "";
    if (!text) return;

    try {
      await navigator.clipboard.writeText(text);
      btn.textContent = "Copied";
      setTimeout(() => (btn.textContent = "Copy"), 900);
    } catch {
      // Fallback
      window.prompt("Copy to clipboard:", text);
    }
  });

  btnDownload.addEventListener("click", () => {
    const busId = (busIdEl.value || "").trim() || "bus";
    const data = {
      apiBaseUrl: normalizeBaseUrl(baseUrlEl.value),
      busId,
      mqtt: {
        username: outUserEl.textContent || "",
        password: outPassEl.textContent || "",
        topic: outTopicEl.textContent || "",
      },
      gpsPayloadTemplate: JSON.parse(payloadTemplate(busId)),
      notes: "Password is one-time; store securely. Do not commit this file to git.",
    };
    downloadJson(`alebus-device-${busId}.json`, data);
  });

  let passwordHidden = false;
  btnHidePassword.addEventListener("click", () => {
    const current = outPassEl.textContent || "";
    if (!current) return;

    if (!passwordHidden) {
      outPassEl.setAttribute("data-real", current);
      outPassEl.textContent = maskPassword(current);
      btnHidePassword.textContent = "Show password";
      passwordHidden = true;
      return;
    }

    const real = outPassEl.getAttribute("data-real") || "";
    outPassEl.textContent = real;
    btnHidePassword.textContent = "Hide password";
    passwordHidden = false;
  });

  // sensible defaults
  loadState();
  baseUrlEl.value = normalizeBaseUrl(baseUrlEl.value);
  if (!busIdEl.value) busIdEl.value = "ANDROID-001";
  payloadEl.textContent = payloadTemplate((busIdEl.value || "").trim());

	if (gpsLatEl && !gpsLatEl.value) gpsLatEl.value = "-20.2748";
	if (gpsLonEl && !gpsLonEl.value) gpsLonEl.value = "57.3669";
	if (gpsSpeedEl && !gpsSpeedEl.value) gpsSpeedEl.value = "10";
	if (gpsHeadingEl && !gpsHeadingEl.value) gpsHeadingEl.value = "180";
	setStatus(gpsStatusEl, null, "Ready.");

  setStatus(opStatusEl, null, "Ready.");
})();
