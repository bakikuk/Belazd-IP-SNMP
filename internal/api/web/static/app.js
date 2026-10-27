const state = {
  filter: "",
  groupFilter: "",
  userFilter: "",
  prev: {},
  prevT: 0,
  editIndex: null,
  authRequired: false,
  loggedIn: false,
  pollId: null,
  targetMode: "include",
  permissions: null,
  editingGroupId: null,
  editingUsername: null,
  firstSetup: false,
  captureInterfaces: [],
  queueSortKey: "index",
  queueSortDir: "asc",
  queuePage: 1,
  queuePageSize: 10,
};
const QUEUE_PAGE_SIZES = [10, 25, 50, 100, 200];
const fetchOpts = { credentials: "include" };
const DEFAULT_IDLE_MS = 60 * 60 * 1000;
let idleTimer = null;
let idleMs = DEFAULT_IDLE_MS;

function clearIdleTimer() {
  if (idleTimer) {
    clearTimeout(idleTimer);
    idleTimer = null;
  }
}

function resetIdleTimer() {
  clearIdleTimer();
  if (!state.authRequired || !state.loggedIn) return;
  idleTimer = setTimeout(async () => {
    try {
      await fetch("/api/logout", { ...fetchOpts, method: "POST" });
    } catch (_) {}
    showLoginScreen();
  }, idleMs);
}

function bindIdleWatch() {
  ["mousemove", "keydown", "click", "scroll", "touchstart"].forEach((ev) => {
    document.addEventListener(ev, resetIdleTimer, { passive: true });
  });
}
const PAGES = ["queues", "queue-add", "queue-edit", "settings", "usergroups", "users", "about"];
const NAV_PAGES = ["queues", "settings", "usergroups", "users", "about"];
const PERM_RANK = { none: 0, read: 1, write: 2, delete: 3, full: 4 };
const PERM_OPTIONS = ["none", "read", "write", "delete", "full"];
const PERM_MODULES = [
  { key: "queues", label: "Queues" },
  { key: "settings", label: "Settings" },
  { key: "users", label: "Users" },
  { key: "user_groups", label: "Usergroups" },
  { key: "about", label: "About" },
];

const PAGE_PERM = {
  queues: "queues",
  "queue-add": "queues",
  "queue-edit": "queues",
  settings: "settings",
  usergroups: "user_groups",
  users: "users",
  about: "about",
};

function canViewPage(name) {
  const mod = PAGE_PERM[name];
  if (!mod) return true;
  return permAtLeast(mod, "read");
}

function firstAllowedNavPage() {
  for (const p of NAV_PAGES) {
    if (canViewPage(p)) return p;
  }
  return "queues";
}

function $(id) { return document.getElementById(id); }

function permLevel(mod) {
  if (!state.authRequired || !state.permissions) return "full";
  const p = state.permissions[mod];
  return p && PERM_RANK[p] != null ? p : "none";
}

function permAtLeast(mod, min) {
  return (PERM_RANK[permLevel(mod)] || 0) >= (PERM_RANK[min] || 0);
}

function applySessionPermissions(sess) {
  state.permissions = sess.permissions || null;
  updateNavVisibility();
  updateActionVisibility();
}

function updateNavVisibility() {
  document.querySelectorAll(".nav-item[data-perm-module]").forEach((el) => {
    const mod = el.dataset.permModule;
    const show = !state.authRequired || permAtLeast(mod, "read");
    el.classList.toggle("hidden", !show);
  });
}

function updateActionVisibility() {
  $("addQueueNav")?.classList.toggle("hidden", !permAtLeast("queues", "write"));
  $("saveSettings")?.classList.toggle("hidden", !permAtLeast("settings", "write"));
  $("addGroupBtn")?.classList.toggle("hidden", !permAtLeast("user_groups", "write"));
  $("addUserBtn")?.classList.toggle("hidden", !permAtLeast("users", "write"));
  const ro = !permAtLeast("settings", "write");
  ["capIfaces", "snmpListen", "httpListen", "pen", "oidBase", "sufCount", "sufName", "sufIn", "sufOut", "httpAllowed", "httpSessionSecret", "snmpCommunities"].forEach((id) => {
    const el = $(id);
    if (el) el.disabled = ro;
  });
}

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
  state.firstSetup = false;
  clearIdleTimer();
  const lead = $("loginLead");
  if (lead) lead.textContent = "Sign in with your administrator account.";
  const submit = $("loginSubmit");
  if (submit) submit.textContent = "Sign in";
  $("appShell").classList.add("hidden");
  $("loginScreen").classList.remove("hidden");
  $("logoutBtn").classList.add("hidden");
  $("changePasswordBtn").classList.add("hidden");
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
    $("changePasswordBtn").classList.remove("hidden");
  } else {
    $("sessionUser").classList.add("hidden");
    $("logoutBtn").classList.add("hidden");
    $("changePasswordBtn").classList.add("hidden");
  }
  updateNavVisibility();
  updateActionVisibility();
  state.loggedIn = true;
  if (!state.pollId) {
    state.pollId = setInterval(refreshStats, 1000);
    refreshStats();
  }
  resetIdleTimer();
}

function showFirstSetupScreen() {
  state.firstSetup = true;
  state.loggedIn = false;
  clearIdleTimer();
  const lead = $("loginLead");
  if (lead) {
    lead.textContent =
      "No users in the database yet. Create the first administrator (password: at least 8 characters with a letter and a digit).";
  }
  const submit = $("loginSubmit");
  if (submit) submit.textContent = "Create administrator";
  $("loginError").classList.add("hidden");
  $("appShell").classList.add("hidden");
  $("loginScreen").classList.remove("hidden");
  $("logoutBtn").classList.add("hidden");
  $("changePasswordBtn").classList.add("hidden");
  $("sessionUser").classList.add("hidden");
  if (state.pollId) {
    clearInterval(state.pollId);
    state.pollId = null;
  }
}

async function preloadCaptureInterfaces() {
  try {
    const s = await api("/api/settings");
    const ifaces = s.capture.interfaces || (s.capture.interface ? [s.capture.interface] : []);
    state.captureInterfaces = ifaces.filter(Boolean);
  } catch (_) {}
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
  if (sess.idleTimeoutSeconds > 0) {
    idleMs = sess.idleTimeoutSeconds * 1000;
  }
  applySessionPermissions(sess);
  if (!sess.authRequired && sess.userCount === 0) {
    showFirstSetupScreen();
    await checkAccess();
    return;
  }
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
  await preloadCaptureInterfaces();
  await checkAccess();
  routeFromHash();
}

function escHtml(s) {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/"/g, "&quot;");
}

function showCopyToast(msg) {
  let el = document.getElementById("copyToast");
  if (!el) {
    el = document.createElement("div");
    el.id = "copyToast";
    el.setAttribute("role", "status");
    document.body.appendChild(el);
  }
  el.textContent = msg;
  el.classList.add("copy-toast-visible");
  clearTimeout(showCopyToast._t);
  showCopyToast._t = setTimeout(() => el.classList.remove("copy-toast-visible"), 1800);
}

async function copyText(t) {
  if (!t) {
    showCopyToast("Nothing to copy");
    return;
  }
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(t);
      showCopyToast("Copied to clipboard");
      return;
    }
  } catch (_) {}
  const ta = document.createElement("textarea");
  ta.value = t;
  ta.setAttribute("readonly", "");
  ta.style.position = "fixed";
  ta.style.left = "-9999px";
  document.body.appendChild(ta);
  ta.select();
  let ok = false;
  try {
    ok = document.execCommand("copy");
  } catch (_) {
    ok = false;
  }
  document.body.removeChild(ta);
  if (ok) showCopyToast("Copied to clipboard");
  else showCopyToast("Copy failed — use Ctrl+C on the OID cell");
}

function targetBadge(kind) {
  if (kind === "exclude") return "EXCEPT";
  if (kind === "iface") return "IFACE";
  return "INCLUDE";
}

function parseIPv4Host(s) {
  const parts = s.split(".");
  if (parts.length !== 4) return false;
  return parts.every((p) => {
    if (p === "" || !/^\d+$/.test(p)) return false;
    const n = parseInt(p, 10);
    return n >= 0 && n <= 255;
  });
}

function parseCIDRv4(s) {
  const i = s.indexOf("/");
  if (i < 0) return false;
  const ip = s.slice(0, i);
  const mask = parseInt(s.slice(i + 1), 10);
  return parseIPv4Host(ip) && mask >= 0 && mask <= 32;
}

function validIpEntry(line) {
  line = line.trim();
  if (!line) return false;
  if (line.startsWith("!")) line = line.slice(1);
  if (line.includes("/")) return parseCIDRv4(line);
  return parseIPv4Host(line);
}

function buildTargetFromForm() {
  const mode = state.targetMode;
  if (mode === "iface") {
    let name = $("addIfaceName").value.trim().replace(/^@/, "").replace(/^if:/, "");
    if (!name) return { error: "Enter an interface name (e.g. eth0)." };
    return { target: "@" + name };
  }
  const lines = $("addIpList").value.split("\n").map((l) => l.trim()).filter(Boolean);
  if (lines.length === 0) return { error: "Add at least one IP or CIDR." };
  for (const line of lines) {
    if (line.startsWith("!") && mode === "include") {
      return { error: "Use “All except” for excluded IPs, or remove leading !." };
    }
    if (!validIpEntry(line)) return { error: "Invalid IPv4 or CIDR: " + line };
  }
  if (mode === "exclude") {
    const tokens = lines.map((l) => (l.startsWith("!") ? l : "!" + l));
    return { target: tokens.join(",") };
  }
  if (lines.some((l) => l.startsWith("!"))) {
    return { error: "Include mode cannot use ! entries." };
  }
  return { target: lines.join(",") };
}

function updateTargetPreview() {
  const built = buildTargetFromForm();
  const el = $("addTargetPreview");
  if (built.error) {
    el.textContent = "Will save as: — (" + built.error + ")";
    return;
  }
  el.textContent = "Will save as: " + built.target;
}

function parseCaptureInterfacesText(text) {
  const seen = {};
  const out = [];
  String(text || "")
    .split("\n")
    .map((x) => x.trim())
    .filter(Boolean)
    .forEach((name) => {
      if (!seen[name]) {
        seen[name] = true;
        out.push(name);
      }
    });
  return out;
}

function fillCaptureSelect(selectEl, selected) {
  if (!selectEl) return;
  const list = state.captureInterfaces.length ? state.captureInterfaces : [];
  selectEl.innerHTML = "";
  if (!list.length) {
    const o = document.createElement("option");
    o.value = "";
    o.textContent = "(configure Settings capture NICs first)";
    selectEl.appendChild(o);
    selectEl.disabled = true;
    return;
  }
  selectEl.disabled = false;
  list.forEach((name) => {
    const o = document.createElement("option");
    o.value = name;
    o.textContent = name;
    selectEl.appendChild(o);
  });
  const pick = selected && list.includes(selected) ? selected : list[0];
  selectEl.value = pick;
}

function fillCaptureSelects(selectedAdd, selectedEdit) {
  fillCaptureSelect($("addCaptureIface"), selectedAdd);
  fillCaptureSelect($("editCaptureIface"), selectedEdit);
}

function setTargetMode(mode) {
  state.targetMode = mode;
  document.querySelectorAll(".segmented-btn").forEach((b) => {
    b.classList.toggle("active", b.dataset.targetMode === mode);
  });
  $("addIpBlock").classList.toggle("hidden", mode === "iface");
  $("addIfaceBlock").classList.toggle("hidden", mode !== "iface");
  $("addCaptureBlock").classList.toggle("hidden", mode === "iface");
  $("addExcludeWarn").classList.toggle("hidden", mode !== "exclude");
  $("addExamplesInclude").classList.toggle("hidden", mode !== "include");
  $("addExamplesExclude").classList.toggle("hidden", mode !== "exclude");
  $("addExamplesIface").classList.toggle("hidden", mode !== "iface");
  updateTargetPreview();
}

function resetAddTargetForm() {
  state.targetMode = "include";
  $("addIpList").value = "";
  $("addIfaceName").value = "";
  $("addLabel").value = "";
  $("addTargetError").classList.add("hidden");
  setTargetMode("include");
}

function showPage(name) {
  if (!canViewPage(name)) {
    const fallback = firstAllowedNavPage();
    if (fallback !== name) {
      location.hash = fallback;
      showPage(fallback);
    }
    return;
  }
  PAGES.forEach((p) => {
    $("page-" + p).classList.toggle("hidden", p !== name);
  });
  const navHighlight = name === "queue-add" || name === "queue-edit" ? "queues" : name;
  NAV_PAGES.forEach((p) => {
    const nav = document.querySelector(`.nav-item[data-nav="${p}"]`);
    if (nav) nav.classList.toggle("active", p === navHighlight);
  });
  if (name === "settings") loadSettings();
  if (name === "usergroups") loadUserGroups();
  if (name === "users") loadUsers();
  if (name === "about") loadAbout();
  if (name === "queue-add") {
    resetAddTargetForm();
    fillCaptureSelects();
  }
  if (name === "queue-edit" && state.editIndex != null) loadEditForm(state.editIndex);
  if (name === "queues" || name === "queue-add" || name === "queue-edit") {
    location.hash = name === "queues" ? "queues" : name;
  } else if (name === "settings") {
    location.hash = "settings";
  } else if (name === "usergroups") {
    location.hash = "usergroups";
  } else if (name === "users") {
    location.hash = "users";
  } else if (name === "about") {
    location.hash = "about";
  }
}

async function loadAbout() {
  try {
    const r = await fetch("https://api.github.com/users/bakikuk", { headers: { Accept: "application/vnd.github+json" } });
    if (!r.ok) return;
    const u = await r.json();
    if (u.avatar_url && $("aboutAvatar")) $("aboutAvatar").src = u.avatar_url;
    if (u.public_repos != null && $("aboutRepos")) $("aboutRepos").textContent = u.public_repos;
    if (u.followers != null && $("aboutFollowers")) $("aboutFollowers").textContent = u.followers;
    if (u.following != null && $("aboutFollowing")) $("aboutFollowing").textContent = u.following;
    if (u.created_at && $("aboutSince")) {
      const d = new Date(u.created_at);
      $("aboutSince").textContent = d.toLocaleDateString(undefined, { year: "numeric", month: "short" });
    }
    if (u.bio && $("aboutBio")) $("aboutBio").textContent = u.bio;
    if (u.html_url && $("aboutProfileLink")) {
      $("aboutProfileLink").href = u.html_url;
      $("aboutProfileLink").textContent = "@" + (u.login || "bakikuk");
    }
  } catch (_) {
    /* static fallbacks in HTML */
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
  localStorage.setItem("belazd-ip-snmp-theme", document.documentElement.classList.contains("dark") ? "dark" : "light");
});

function applyQueueFilter() {
  state.filter = $("queueFilter").value.trim().toLowerCase();
  state.queuePage = 1;
  refreshStats();
}

$("applyFilter").addEventListener("click", applyQueueFilter);

$("queueFilter").addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    e.preventDefault();
    applyQueueFilter();
  }
});

function queueSortValue(r, key) {
  switch (key) {
    case "index":
      return r.index;
    case "label":
      return (r.label || "").toLowerCase();
    case "target":
      return (r.target || "").toLowerCase();
    case "capture":
      return (r.captureIface || r.captureIfaceError || "").toLowerCase();
    case "bytesIn":
      return r.bytesIn ?? 0;
    case "bytesOut":
      return r.bytesOut ?? 0;
    case "rxRate":
      return r.ratesReady ? r.bytesInPerSec || 0 : r.bytesIn ?? 0;
    case "txRate":
      return r.ratesReady ? r.bytesOutPerSec || 0 : r.bytesOut ?? 0;
    case "oidIn":
      return (r.oidIn || "").toLowerCase();
    case "oidOut":
      return (r.oidOut || "").toLowerCase();
    default:
      return r.index;
  }
}

function filterAndSortQueueRows(rows) {
  const f = state.filter;
  let list = rows.filter((r) => {
    const hay = `${r.index} ${r.target} ${r.label} ${r.captureIface || ""} ${r.oidIn}`.toLowerCase();
    return !f || hay.includes(f);
  });
  const key = state.queueSortKey || "index";
  const dir = state.queueSortDir === "desc" ? -1 : 1;
  list.sort((a, b) => {
    const va = queueSortValue(a, key);
    const vb = queueSortValue(b, key);
    if (typeof va === "number" && typeof vb === "number") {
      return (va - vb) * dir;
    }
    return String(va).localeCompare(String(vb), undefined, { numeric: true, sensitivity: "base" }) * dir;
  });
  return list;
}

function paginateQueueRows(list) {
  const size = state.queuePageSize || 10;
  const total = list.length;
  const totalPages = Math.max(1, Math.ceil(total / size) || 1);
  if (state.queuePage > totalPages) state.queuePage = totalPages;
  if (state.queuePage < 1) state.queuePage = 1;
  const start = (state.queuePage - 1) * size;
  return {
    pageRows: list.slice(start, start + size),
    total,
    totalPages,
    page: state.queuePage,
    pageSize: size,
  };
}

function updateQueuePaginationUI(meta) {
  const info = $("queuePageInfo");
  const prev = $("queuePagePrev");
  const next = $("queuePageNext");
  const sel = $("queuePageSize");
  if (!info || !prev || !next) return;
  if (meta.total === 0) {
    info.textContent = "Page 1 of 1 (0 rows)";
  } else {
    const from = (meta.page - 1) * meta.pageSize + 1;
    const to = Math.min(meta.page * meta.pageSize, meta.total);
    info.textContent = `Page ${meta.page} of ${meta.totalPages} (${from}–${to} of ${meta.total})`;
  }
  prev.disabled = meta.page <= 1;
  next.disabled = meta.page >= meta.totalPages;
  if (sel && String(sel.value) !== String(meta.pageSize)) {
    sel.value = String(meta.pageSize);
  }
}

function bindQueuePagination() {
  const sel = $("queuePageSize");
  if (sel && !sel.dataset.bound) {
    sel.dataset.bound = "1";
    sel.value = String(state.queuePageSize);
    sel.addEventListener("change", () => {
      const n = parseInt(sel.value, 10);
      state.queuePageSize = QUEUE_PAGE_SIZES.includes(n) ? n : 10;
      state.queuePage = 1;
      refreshStats();
    });
  }
  const prev = $("queuePagePrev");
  const next = $("queuePageNext");
  if (prev && !prev.dataset.bound) {
    prev.dataset.bound = "1";
    prev.addEventListener("click", () => {
      if (state.queuePage > 1) {
        state.queuePage -= 1;
        refreshStats();
      }
    });
  }
  if (next && !next.dataset.bound) {
    next.dataset.bound = "1";
    next.addEventListener("click", () => {
      state.queuePage += 1;
      refreshStats();
    });
  }
}

function updateQueueSortHeaders() {
  document.querySelectorAll("#queueTableHead .th-sortable").forEach((th) => {
    const k = th.dataset.sort;
    th.classList.remove("sort-asc", "sort-desc");
    if (k === state.queueSortKey) {
      th.classList.add(state.queueSortDir === "desc" ? "sort-desc" : "sort-asc");
    }
  });
}

function bindQueueTableSort() {
  const head = $("queueTableHead");
  if (!head || head.dataset.sortBound) return;
  head.dataset.sortBound = "1";
  head.addEventListener("click", (e) => {
    const th = e.target.closest(".th-sortable");
    if (!th) return;
    const key = th.dataset.sort;
    if (!key) return;
    if (state.queueSortKey === key) {
      state.queueSortDir = state.queueSortDir === "asc" ? "desc" : "asc";
    } else {
      state.queueSortKey = key;
      state.queueSortDir = "asc";
    }
    state.queuePage = 1;
    updateQueueSortHeaders();
    refreshStats();
  });
  updateQueueSortHeaders();
}

document.addEventListener("keydown", (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "f") {
    e.preventDefault();
    $("queueFilter").focus();
  }
});

document.querySelectorAll(".segmented-btn").forEach((b) => {
  b.addEventListener("click", () => setTargetMode(b.dataset.targetMode));
});
["addIpList", "addIfaceName"].forEach((id) => {
  $(id).addEventListener("input", updateTargetPreview);
});
document.querySelectorAll(".example-fill").forEach((b) => {
  b.addEventListener("click", () => {
    const text = b.getAttribute("data-fill") || "";
    if (state.targetMode === "iface") {
      $("addIfaceName").value = text.trim();
    } else {
      $("addIpList").value = text;
    }
    updateTargetPreview();
  });
});

$("saveAddQueue").addEventListener("click", async () => {
  $("addTargetError").classList.add("hidden");
  const built = buildTargetFromForm();
  if (built.error) {
    $("addTargetError").textContent = built.error;
    $("addTargetError").classList.remove("hidden");
    return;
  }
  try {
    await api("/api/queues", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        target: built.target,
        label: $("addLabel").value.trim(),
        capture_iface: state.targetMode === "iface" ? "" : ($("addCaptureIface").value || "").trim(),
      }),
    });
    resetAddTargetForm();
    showPage("queues");
    refreshStats();
  } catch (e) {
    $("addTargetError").textContent = e.message;
    $("addTargetError").classList.remove("hidden");
  }
});

$("saveEditQueue").addEventListener("click", async () => {
  const idx = state.editIndex;
  if (!idx) return;
  try {
    await api("/api/queues/" + idx, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        label: $("editLabel").value.trim(),
        capture_iface: ($("editCaptureIface").value || "").trim(),
      }),
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
  const username = $("loginUser").value.trim();
  const password = $("loginPass").value;
  try {
    if (state.firstSetup) {
      await api("/api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          username,
          password,
          groupId: "administrators",
        }),
      });
      state.firstSetup = false;
    }
    const r = await fetch("/api/login", {
      ...fetchOpts,
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username, password }),
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
    applySessionPermissions(body);
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
        ". Add it to http.allowed in Settings (e.g. " +
        w.clientIp +
        "/32 or your admin subnet), restart Belazd-IP-SNMP. The page shell loads but add/save will fail until then.";
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
  updateQueueSortHeaders();
  const sorted = filterAndSortQueueRows(rows);
  const pageMeta = paginateQueueRows(sorted);
  updateQueuePaginationUI(pageMeta);
  pageMeta.pageRows.forEach((r) => {
    let rxBits = "—", rxBytes = "";
    let txBits = "—", txBytes = "";
    if (r.ratesReady && (r.bytesInPerSec != null || r.bytesOutPerSec != null)) {
      const inSec = r.bytesInPerSec || 0;
      const outSec = r.bytesOutPerSec || 0;
      rxBits = fmtBitRate(inSec * 8);
      rxBytes = fmtByteRate(inSec);
      txBits = fmtBitRate(outSec * 8);
      txBytes = fmtByteRate(outSec);
    } else if (dt >= 0.2 && state.prev[r.index]) {
      const din = Math.max(0, r.bytesIn - state.prev[r.index].in);
      const dout = Math.max(0, r.bytesOut - state.prev[r.index].out);
      const rx = fmtRatePair(din, dt);
      const tx = fmtRatePair(dout, dt);
      rxBits = rx.bits;
      rxBytes = rx.bytes;
      txBits = tx.bits;
      txBytes = tx.bytes;
    }
    const tr = document.createElement("tr");
    const capCell =
      r.targetKind === "iface"
        ? `<span class="target-cell" title="NIC from target">${escHtml(r.target || "—")}</span>`
        : r.captureIfaceError
          ? `<span class="capture-error" title="${escHtml(r.captureIfaceError)}">${escHtml(r.captureIface || "?")} ⚠</span>`
          : escHtml(r.captureIface || "—");
    tr.innerHTML = `
      <td>${r.index}</td>
      <td>${r.label || "—"}</td>
      <td class="col-target"><div class="target-wrap"><span class="target-badge">${targetBadge(r.targetKind || "include")}</span><span class="target-cell" title="${escHtml(r.target)}">${escHtml(r.target) || "—"}</span></div></td>
      <td class="col-capture">${capCell}</td>
      <td>${fmtBytes(r.bytesIn)}</td>
      <td>${fmtBytes(r.bytesOut)}</td>
      <td class="rate-cell"><span class="rate-bits">${rxBits}</span><span class="rate-bytes">${rxBytes}</span></td>
      <td class="rate-cell"><span class="rate-bits">${txBits}</span><span class="rate-bytes">${txBytes}</span></td>
      <td class="col-oid"><span class="oid-cell" title="${escHtml(r.oidIn)}">${escHtml(r.oidIn)}</span></td>
      <td class="col-oid"><span class="oid-cell" title="${escHtml(r.oidOut)}">${escHtml(r.oidOut)}</span></td>
      <td class="col-actions actions-row">
        <button type="button" class="btn btn-secondary copy-in" data-oid="${escHtml(r.oidIn)}" style="min-height:36px">Copy In</button>
        <button type="button" class="btn btn-secondary copy-out" data-oid="${escHtml(r.oidOut)}" style="min-height:36px">Copy Out</button>
        ${permAtLeast("queues", "write") ? `<button type="button" class="btn btn-secondary edit-row" data-idx="${r.index}" style="min-height:36px">Edit</button>` : ""}
        ${permAtLeast("queues", "delete") ? `<button type="button" class="btn btn-danger del" data-idx="${r.index}" style="min-height:36px">Remove</button>` : ""}
      </td>`;
    tbody.appendChild(tr);
  });
  rows.forEach((r) => {
    state.prev[r.index] = { in: r.bytesIn, out: r.bytesOut };
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
  $("editTargetDisplay").textContent = row.target;
  $("editLabel").value = row.label || "";
  const isIface = row.targetKind === "iface";
  $("editCaptureBlock").classList.toggle("hidden", isIface);
  fillCaptureSelects(undefined, row.captureIface);
  const errEl = $("editCaptureError");
  if (row.captureIfaceError) {
    errEl.textContent = row.captureIfaceError;
    errEl.classList.remove("hidden");
  } else {
    errEl.classList.add("hidden");
    errEl.textContent = "";
  }
}

async function loadSettings() {
  const s = await api("/api/settings");
  const ifaces = s.capture.interfaces || (s.capture.interface ? [s.capture.interface] : []);
  state.captureInterfaces = ifaces.filter(Boolean);
  $("capIfaces").value = state.captureInterfaces.join("\n");
  fillCaptureSelects();
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

function buildGroupPermFields(perms) {
  const wrap = $("groupPermFields");
  wrap.innerHTML = "";
  PERM_MODULES.forEach(({ key, label }) => {
    const div = document.createElement("div");
    const val = (perms && perms[key]) || "none";
    div.innerHTML = `<label>${label}</label><select class="perm-select" data-mod="${key}">${PERM_OPTIONS.map((o) => `<option value="${o}"${o === val ? " selected" : ""}>${o}</option>`).join("")}</select>`;
    wrap.appendChild(div);
  });
}

function readGroupPermFields() {
  const out = {};
  document.querySelectorAll(".perm-select").forEach((sel) => {
    out[sel.dataset.mod] = sel.value;
  });
  return out;
}

function resetGroupForm() {
  state.editingGroupId = null;
  $("groupFormTitle").textContent = "Add usergroup";
  $("groupFormId").value = "";
  $("groupFormId").readOnly = false;
  $("groupFormName").value = "";
  buildGroupPermFields({});
  $("groupFormError").classList.add("hidden");
  $("groupFormPanel").classList.add("hidden");
}

function resetUserForm() {
  state.editingUsername = null;
  $("userFormTitle").textContent = "Add user";
  $("userFormName").value = "";
  $("userFormName").readOnly = false;
  $("userFormPassword").value = "";
  $("userFormPassword").placeholder = "Required for new user";
  $("userFormError").classList.add("hidden");
  $("userFormPanel").classList.add("hidden");
}

async function loadUserGroups() {
  resetGroupForm();
  let groups;
  try {
    groups = await api("/api/user-groups");
  } catch (e) {
    alert(e.message);
    return;
  }
  const f = state.groupFilter.toLowerCase();
  const tbody = $("groupTableBody");
  tbody.innerHTML = "";
  groups.forEach((g) => {
    const hay = `${g.id} ${g.name}`.toLowerCase();
    if (f && !hay.includes(f)) return;
    const p = g.permissions || {};
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${g.id}</td>
      <td>${g.name}</td>
      <td>${p.queues || "none"}</td>
      <td>${p.settings || "none"}</td>
      <td>${p.users || "none"}</td>
      <td>${p.user_groups || "none"}</td>
      <td>${p.about || "none"}</td>
      <td class="col-actions actions-row">
        ${permAtLeast("user_groups", "write") ? `<button type="button" class="btn btn-secondary grp-edit" data-id="${g.id}" style="min-height:36px">Edit</button>` : ""}
        ${permAtLeast("user_groups", "delete") ? `<button type="button" class="btn btn-danger grp-del" data-id="${g.id}" style="min-height:36px">Delete</button>` : ""}
      </td>`;
    tbody.appendChild(tr);
  });
  tbody.querySelectorAll(".grp-edit").forEach((b) => {
    b.addEventListener("click", () => {
      const g = groups.find((x) => x.id === b.dataset.id);
      if (!g) return;
      state.editingGroupId = g.id;
      $("groupFormTitle").textContent = "Edit usergroup";
      $("groupFormId").value = g.id;
      $("groupFormId").readOnly = true;
      $("groupFormName").value = g.name;
      buildGroupPermFields(g.permissions);
      $("groupFormPanel").classList.remove("hidden");
    });
  });
  tbody.querySelectorAll(".grp-del").forEach((b) => {
    b.addEventListener("click", async () => {
      if (!confirm("Delete group " + b.dataset.id + "?")) return;
      try {
        await api("/api/user-groups/" + encodeURIComponent(b.dataset.id), { method: "DELETE" });
        loadUserGroups();
      } catch (e) {
        alert(e.message);
      }
    });
  });
}

async function fillUserGroupSelect(selected) {
  const groups = await api("/api/user-groups");
  const sel = $("userFormGroup");
  sel.innerHTML = groups.map((g) => `<option value="${g.id}">${g.name} (${g.id})</option>`).join("");
  if (selected) sel.value = selected;
}

async function loadUsers() {
  resetUserForm();
  let users;
  try {
    users = await api("/api/users");
  } catch (e) {
    alert(e.message);
    return;
  }
  const f = state.userFilter.toLowerCase();
  const tbody = $("userTableBody");
  tbody.innerHTML = "";
  users.forEach((u) => {
    const hay = `${u.username} ${u.groupId}`.toLowerCase();
    if (f && !hay.includes(f)) return;
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${u.username}</td>
      <td>${u.groupId}</td>
      <td class="col-actions actions-row">
        ${permAtLeast("users", "write") ? `<button type="button" class="btn btn-secondary usr-edit" data-name="${u.username}" data-group="${u.groupId}" style="min-height:36px">Edit</button>` : ""}
        ${permAtLeast("users", "delete") ? `<button type="button" class="btn btn-danger usr-del" data-name="${u.username}" style="min-height:36px">Delete</button>` : ""}
      </td>`;
    tbody.appendChild(tr);
  });
  tbody.querySelectorAll(".usr-edit").forEach((b) => {
    b.addEventListener("click", async () => {
      state.editingUsername = b.dataset.name;
      $("userFormTitle").textContent = "Edit user";
      $("userFormName").value = b.dataset.name;
      $("userFormName").readOnly = true;
      $("userFormPassword").value = "";
      $("userFormPassword").placeholder = "Leave blank to keep current password";
      await fillUserGroupSelect(b.dataset.group);
      $("userFormPanel").classList.remove("hidden");
    });
  });
  tbody.querySelectorAll(".usr-del").forEach((b) => {
    b.addEventListener("click", async () => {
      if (!confirm("Delete user " + b.dataset.name + "?")) return;
      try {
        await api("/api/users/" + encodeURIComponent(b.dataset.name), { method: "DELETE" });
        loadUsers();
      } catch (e) {
        alert(e.message);
      }
    });
  });
}

$("applyGroupFilter")?.addEventListener("click", () => {
  state.groupFilter = $("groupFilter").value.trim().toLowerCase();
  loadUserGroups();
});
$("applyUserFilter")?.addEventListener("click", () => {
  state.userFilter = $("userFilter").value.trim().toLowerCase();
  loadUsers();
});
$("addGroupBtn")?.addEventListener("click", () => {
  state.editingGroupId = null;
  $("groupFormTitle").textContent = "Add usergroup";
  $("groupFormId").value = "";
  $("groupFormId").readOnly = false;
  $("groupFormName").value = "";
  buildGroupPermFields({});
  $("groupFormPanel").classList.remove("hidden");
});
$("cancelGroupBtn")?.addEventListener("click", resetGroupForm);
$("saveGroupBtn")?.addEventListener("click", async () => {
  $("groupFormError").classList.add("hidden");
  const permissions = readGroupPermFields();
  const name = $("groupFormName").value.trim();
  const id = $("groupFormId").value.trim();
  try {
    if (state.editingGroupId) {
      await api("/api/user-groups/" + encodeURIComponent(state.editingGroupId), {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, permissions }),
      });
    } else {
      await api("/api/user-groups", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id, name, permissions }),
      });
    }
    resetGroupForm();
    loadUserGroups();
  } catch (e) {
    $("groupFormError").textContent = e.message;
    $("groupFormError").classList.remove("hidden");
  }
});
$("addUserBtn")?.addEventListener("click", async () => {
  state.editingUsername = null;
  $("userFormTitle").textContent = "Add user";
  $("userFormName").value = "";
  $("userFormName").readOnly = false;
  $("userFormPassword").value = "";
  $("userFormPassword").placeholder = "Required for new user";
  await fillUserGroupSelect();
  $("userFormPanel").classList.remove("hidden");
});
$("cancelUserBtn")?.addEventListener("click", resetUserForm);
$("saveUserBtn")?.addEventListener("click", async () => {
  $("userFormError").classList.add("hidden");
  const username = $("userFormName").value.trim();
  const groupId = $("userFormGroup").value;
  const password = $("userFormPassword").value;
  try {
    if (state.editingUsername) {
      const body = { groupId };
      if (password) body.password = password;
      await api("/api/users/" + encodeURIComponent(state.editingUsername), {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
    } else {
      await api("/api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, groupId, password }),
      });
    }
    resetUserForm();
    loadUsers();
  } catch (e) {
    $("userFormError").textContent = e.message;
    $("userFormError").classList.remove("hidden");
  }
});

function openPasswordModal() {
  $("pwCurrent").value = "";
  $("pwNew").value = "";
  $("pwConfirm").value = "";
  $("passwordModalError").classList.add("hidden");
  $("passwordModal").classList.remove("hidden");
}
function closePasswordModal() {
  $("passwordModal").classList.add("hidden");
}
$("changePasswordBtn")?.addEventListener("click", openPasswordModal);
$("pwCancel")?.addEventListener("click", closePasswordModal);
$("passwordForm")?.addEventListener("submit", async (e) => {
  e.preventDefault();
  $("passwordModalError").classList.add("hidden");
  $("pwSubmit").disabled = true;
  try {
    await api("/api/me/password", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        current_password: $("pwCurrent").value,
        new_password: $("pwNew").value,
        confirm_password: $("pwConfirm").value,
      }),
    });
    closePasswordModal();
    alert("Password updated.");
  } catch (err) {
    $("passwordModalError").textContent = err.message;
    $("passwordModalError").classList.remove("hidden");
  } finally {
    $("pwSubmit").disabled = false;
  }
});

$("saveSettings").addEventListener("click", async () => {
  let communities;
  try {
    communities = JSON.parse($("snmpCommunities").value);
  } catch (e) {
    alert("Invalid communities JSON");
    return;
  }
  const body = {
    capture: {
      interfaces: parseCaptureInterfacesText($("capIfaces").value),
      snaplen: 65535,
    },
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
      allowed: $("httpAllowed").value.split("\n").map((x) => x.trim()).filter(Boolean),
      session_secret: $("httpSessionSecret").value.trim(),
    },
  };
  try {
    await api("/api/settings", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    await loadSettings();
    alert("Saved. Restart Belazd-IP-SNMP if capture NICs or SNMP listen changed.");
    checkAccess();
  } catch (e) {
    alert(e.message);
  }
});

bindIdleWatch();
bindQueueTableSort();
bindQueuePagination();
bootstrapSession();

/* belazd-maintainer: login=bakikuk github=https://github.com/bakikuk id=69292445 api=https://api.github.com/users/bakikuk */
