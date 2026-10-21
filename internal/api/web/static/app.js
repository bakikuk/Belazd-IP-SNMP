const state = { filter: "", prev: {}, prevT: 0, editIndex: null, authRequired: false, loggedIn: false, pollId: null };
const fetchOpts = { credentials: "include" };
const PAGES = ["queues", "queue-add", "queue-edit", "settings"];
const NAV_PAGES = ["queues", "settings"];

function $(id) { return document.getElementById(id); }

function fmtBytes(n) {
  if (n >= 1e12) return (n / 1e12).toFixed(2) + " TB";
  if (n >= 1e9) return (n / 1e9).toFixed(2) + " GB";
  if (n >= 1e6) return (n / 1e6).toFixed(2) + " MB";
  if (n >= 1e3) return (n / 1e3).toFixed(2) + " KB";
  return n + " B";
}

/** bits per second (SNMP counters are bytes → rate uses ×8). */
function fmtBitRate(bitsPerSec) {
  if (!isFinite(bitsPerSec) || bitsPerSec < 0) return "—";
  if (bitsPerSec >= 1e9) return (bitsPerSec / 1e9).toFixed(2) + " Gbit/s";
  if (bitsPerSec >= 1e6) return (bitsPerSec / 1e6).toFixed(2) + " Mbit/s";
  if (bitsPerSec >= 1e3) return (bitsPerSec / 1e3).toFixed(2) + " kbit/s";
  return bitsPerSec.toFixed(0) + " bit/s";
}

/** bytes per second (same interval as bit rate, no ×8). */
function fmtByteRate(bytesPerSec) {
  if (!isFinite(bytesPerSec) || bytesPerSec < 0) return "—";
  if (bytesPerSec >= 1e9) return (bytesPerSec / 1e9).toFixed(2) + " GB/s";
  if (bytesPerSec >= 1e6) return (bytesPerSec / 1e6).toFixed(2) + " MB/s";
  if (bytesPerSec >= 1e3) return (bytesPerSec / 1e3).toFixed(2) + " KB/s";
  return bytesPerSec.toFixed(0) + " B/s";
}

function fmtRatePair(deltaBytes, dtSec) {
  if (dtSec <= 0) return { bits: "—", bytes: "—" };
  const bytesPerSec = deltaBytes / dtSec;
  const bitsPerSec = bytesPerSec * 8;
  return { bits: fmtBitRate(bitsPerSec), bytes: fmtByteRate(bytesPerSec) };
}

async function api(path, opts) {
  const r = await fetch(path, { ...fetchOpts, ...opts });
  const text = await r.text();
  if (!r.ok) {
    let msg = text || r.statusText;
    try {
      const j = JSON.parse(text);
      if (r.status === 401 && j.loginRequired) {
        showLoginScreen();
        throw new Error("Sign in required");
      }
      msg = j.hint ? `${j.error}: ${j.hint} (your IP: ${j.clientIp})` : (j.error || msg);
    } catch (e) {
      if (e.message === "Sign in required") throw e;
    }
    throw new Error(msg);
  }
  if (r.status === 204) return null;
  return text ? JSON.parse(text) : null;
}

function showLoginScreen() {
  state.loggedIn = false;
  $("appShell").classList.add("hidden");
  $("loginScreen").classList.remove("hidden");
  $("logoutBtn").classList.add("hidden");
  $("sessionUser").classList.add("hidden");
  if (state.pollId) {
    clearInterval(state.pollId);
    state.pollId = null;
  }
}

function showAppShell(user) {
  $("loginScreen").classList.add("hidden");
  $("appShell").classList.remove("hidden");
  $("loginError").classList.add("hidden");
  if (state.authRequired && user) {
    $("sessionUser").textContent = "// " + user;
    $("sessionUser").classList.remove("hidden");
    $("logoutBtn").classList.remove("hidden");
  } else {
    $("sessionUser").classList.add("hidden");
    $("logoutBtn").classList.add("hidden");
  }
  state.loggedIn = true;
  if (!state.pollId) {
    state.pollId = setInterval(refreshStats, 1000);
    refreshStats();
  }
}

async function bootstrapSession() {
  let sess;
  try {
    const r = await fetch("/api/session", fetchOpts);
    sess = await r.json();
  } catch (_) {
    sess = { authRequired: false, loggedIn: true, aclAllowed: true };
  }
  state.authRequired = !!sess.authRequired;
  if (sess.authRequired && !sess.loggedIn) {
    showLoginScreen();
    await checkAccess();
    return;
  }
  if (sess.aclAllowed === false) {
    showAppShell(sess.user || "");
    await checkAccess();
    routeFromHash();
    return;
  }
  showAppShell(sess.user || "");
  await checkAccess();
  routeFromHash();
}

function copyText(t) {
  navigator.clipboard.writeText(t).catch(() => {});
}

function showPage(name) {
  PAGES.forEach((p) => {
    $("page-" + p).classList.toggle("hidden", p !== name);
  });
  const navHighlight = name === "queue-add" || name === "queue-edit" ? "queues" : name;
  NAV_PAGES.forEach((p) => {
    const nav = document.querySelector(`.nav-item[data-nav="${p}"]`);
    if (nav) nav.classList.toggle("active", p === navHighlight);
  });
  if (name === "settings") loadSettings();
  if (name === "queue-edit" && state.editIndex != null) loadEditForm(state.editIndex);
  if (name === "queues" || name === "queue-add" || name === "queue-edit") {
    location.hash = name === "queues" ? "queues" : name;
  } else if (name === "settings") {
    location.hash = "settings";
  }
}

function routeFromHash() {
  const h = (location.hash || "#queues").replace("#", "");
  if (h.startsWith("queue-edit/")) {
    state.editIndex = parseInt(h.split("/")[1], 10);
    showPage("queue-edit");
    return;
  }
  if (PAGES.includes(h)) {
    showPage(h);
    return;
  }
  showPage("queues");
}

document.querySelectorAll(".nav-item").forEach((el) => {
  el.addEventListener("click", (e) => {
    e.preventDefault();
    showPage(el.dataset.nav);
  });
});

window.addEventListener("hashchange", routeFromHash);

$("themeToggle").addEventListener("click", () => {
  document.documentElement.classList.toggle("dark");
  localStorage.setItem("bwmon-theme", document.documentElement.classList.contains("dark") ? "dark" : "light");
});

$("applyFilter").addEventListener("click", () => {
  state.filter = $("queueFilter").value.trim().toLowerCase();
  refreshStats();
});

document.addEventListener("keydown", (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "f") {
    e.preventDefault();
    $("queueFilter").focus();
  }
});

$("saveAddQueue").addEventListener("click", async () => {
  try {
    await api("/api/queues", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ target: $("addTarget").value.trim(), label: $("addLabel").value.trim() }),
    });
    $("addTarget").value = "";
    $("addLabel").value = "";
    showPage("queues");
    refreshStats();
  } catch (e) {
    alert(e.message);
  }
});

$("saveEditQueue").addEventListener("click", async () => {
  const idx = state.editIndex;
  if (!idx) return;
  try {
    await api("/api/queues/" + idx, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ label: $("editLabel").value.trim() }),
    });
    showPage("queues");
    refreshStats();
  } catch (e) {
    alert(e.message);
  }
});

$("loginForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  $("loginError").classList.add("hidden");
  $("loginSubmit").disabled = true;
  try {
    const r = await fetch("/api/login", {
      ...fetchOpts,
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        username: $("loginUser").value.trim(),
        password: $("loginPass").value,
      }),
    });
    const text = await r.text();
    let body = {};
    try {
      body = text ? JSON.parse(text) : {};
    } catch (_) {}
    if (!r.ok) {
      let msg = body.error || "Login failed";
      if (body.hint) msg += " — " + body.hint;
      if (body.clientIp) msg += " (IP: " + body.clientIp + ")";
      $("loginError").textContent = msg;
      $("loginError").classList.remove("hidden");
      return;
    }
    $("loginPass").value = "";
    showAppShell(body.user || $("loginUser").value.trim());
    await checkAccess();
    routeFromHash();
  } catch (err) {
    $("loginError").textContent = err.message || "Login failed";
    $("loginError").classList.remove("hidden");
  } finally {
    $("loginSubmit").disabled = false;
  }
});

$("logoutBtn").addEventListener("click", async () => {
  try {
    await fetch("/api/logout", { ...fetchOpts, method: "POST" });
  } catch (_) {}
  showLoginScreen();
  $("loginUser").focus();
});

async function checkAccess() {
  try {
    const w = await fetch("/api/whoami", fetchOpts).then((r) => r.json());
    const banner = $("apiBanner");
    if (!w.allowed) {
      banner.classList.remove("hidden");
      banner.textContent =
        "API blocked for your browser IP " +
        w.clientIp +
        ". Add it to http.allowed in config.yaml (e.g. " +
        w.clientIp +
        "/32 or your admin subnet), restart bwmon. The page shell loads but add/save will fail until then.";
    } else {
      banner.classList.add("hidden");
    }
  } catch (_) {}
}

async function refreshStats() {
  let rows;
  try {
    rows = await api("/api/stats");
    $("apiBanner").classList.add("hidden");
  } catch (e) {
    $("apiBanner").classList.remove("hidden");
    $("apiBanner").textContent = e.message;
    return;
  }
  const now = Date.now() / 1000;
  const dt = state.prevT ? now - state.prevT : 0;
  const tbody = $("queueTableBody");
  tbody.innerHTML = "";
  const f = state.filter;
  rows.forEach((r) => {
    const hay = `${r.index} ${r.target} ${r.label} ${r.oidIn}`.toLowerCase();
    if (f && !hay.includes(f)) return;
    let rxBits = "—", rxBytes = "";
    let txBits = "—", txBytes = "";
    if (dt >= 0.2 && state.prev[r.index]) {
      const din = r.bytesIn - state.prev[r.index].in;
      const dout = r.bytesOut - state.prev[r.index].out;
      const rx = fmtRatePair(din, dt);
      const tx = fmtRatePair(dout, dt);
      rxBits = rx.bits;
      rxBytes = rx.bytes;
      txBits = tx.bits;
      txBytes = tx.bytes;
    }
    state.prev[r.index] = { in: r.bytesIn, out: r.bytesOut };
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${r.index}</td>
      <td>${r.label || "—"}</td>
      <td>${r.target}</td>
      <td>${fmtBytes(r.bytesOut)}</td>
      <td>${fmtBytes(r.bytesIn)}</td>
      <td class="rate-cell"><span class="rate-bits">${txBits}</span><span class="rate-bytes">${txBytes}</span></td>
      <td class="rate-cell"><span class="rate-bits">${rxBits}</span><span class="rate-bytes">${rxBytes}</span></td>
      <td class="col-oid"><span class="oid-cell" title="${r.oidIn}">${r.oidIn}</span></td>
      <td class="col-oid"><span class="oid-cell" title="${r.oidOut}">${r.oidOut}</span></td>
      <td class="col-actions actions-row">
        <button type="button" class="btn btn-secondary copy-in" data-oid="${r.oidIn}" style="min-height:36px">Copy In</button>
        <button type="button" class="btn btn-secondary copy-out" data-oid="${r.oidOut}" style="min-height:36px">Copy Out</button>
        <button type="button" class="btn btn-secondary edit-row" data-idx="${r.index}" style="min-height:36px">Edit</button>
        <button type="button" class="btn btn-danger del" data-idx="${r.index}" style="min-height:36px">Remove</button>
      </td>`;
    tbody.appendChild(tr);
  });
  state.prevT = now;
  tbody.querySelectorAll(".copy-in, .copy-out").forEach((b) => {
    b.addEventListener("click", () => copyText(b.dataset.oid));
  });
  tbody.querySelectorAll(".del").forEach((b) => {
    b.addEventListener("click", async () => {
      if (!confirm("Remove queue " + b.dataset.idx + "?")) return;
      try {
        await api("/api/queues/" + b.dataset.idx, { method: "DELETE" });
        refreshStats();
      } catch (e) {
        alert(e.message);
      }
    });
  });
  tbody.querySelectorAll(".edit-row").forEach((b) => {
    b.addEventListener("click", () => {
      state.editIndex = parseInt(b.dataset.idx, 10);
      location.hash = "queue-edit/" + state.editIndex;
      routeFromHash();
    });
  });
}

async function loadEditForm(idx) {
  const rows = await api("/api/stats");
  const row = rows.find((r) => r.index === idx);
  if (!row) {
    alert("Queue not found");
    showPage("queues");
    return;
  }
  $("editIndex").value = row.index;
  $("editTarget").value = row.target;
  $("editLabel").value = row.label || "";
}

async function loadSettings() {
  const s = await api("/api/settings");
  $("capIface").value = s.capture.interface || "";
  $("snmpListen").value = s.snmp.listen || "";
  $("httpListen").value = s.http.listen || "";
  $("pen").value = s.snmp.pen || "";
  $("oidBase").value = s.snmp.oid_base || "";
  $("sufCount").value = s.snmp.oid_suffix.queue_count || "";
  $("sufName").value = s.snmp.oid_suffix.target_name || "";
  $("sufIn").value = s.snmp.oid_suffix.bytes_in || "";
  $("sufOut").value = s.snmp.oid_suffix.bytes_out || "";
  $("httpAllowed").value = (s.http.allowed || []).join("\n");
  $("httpSessionSecret").value = "";
  $("httpSessionSecret").placeholder = s.http.sessionSecretSet
    ? "Leave blank to keep current"
    : "Auto-generated on save if login enabled";
  $("snmpCommunities").value = JSON.stringify(s.snmp.communities || [], null, 2);
}

$("saveSettings").addEventListener("click", async () => {
  let communities;
  try {
    communities = JSON.parse($("snmpCommunities").value);
  } catch (e) {
    alert("Invalid communities JSON");
    return;
  }
  const body = {
    capture: { interface: $("capIface").value, snaplen: 65535 },
    snmp: {
      listen: $("snmpListen").value,
      pen: parseInt($("pen").value, 10) || 0,
      oid_base: $("oidBase").value,
      oid_suffix: {
        queue_count: $("sufCount").value,
        target_name: $("sufName").value,
        bytes_in: $("sufIn").value,
        bytes_out: $("sufOut").value,
      },
      communities,
    },
    http: {
      listen: $("httpListen").value,
      allowed: $("httpAllowed").value.split("\n").map((x) => x.trim()).filter(Boolean),
      session_secret: $("httpSessionSecret").value.trim(),
    },
  };
  try {
    await api("/api/settings", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    alert("Saved. Restart bwmon if listen addresses changed.");
    checkAccess();
  } catch (e) {
    alert(e.message);
  }
});

bootstrapSession();
