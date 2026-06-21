const CIRC = 2 * Math.PI * 44; // 276.46

let state = {
  sites: [],
  presets: [],
  session: { active: false },
  status: null,
};

function $(sel) {
  return document.querySelector(sel);
}

function showToast(msg) {
  const t = $("#toast");
  t.textContent = msg;
  t.classList.add("show");
  clearTimeout(showToast._timer);
  showToast._timer = setTimeout(() => t.classList.remove("show"), 4500);
}

async function api(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  let data = null;
  try {
    data = await res.json();
  } catch (_) {
    /* no body */
  }
  if (!res.ok) {
    const msg = (data && data.error) || `error ${res.status}`;
    throw new Error(msg);
  }
  return data;
}

async function loadAll() {
  const [status, sites, presets, session] = await Promise.all([
    api("GET", "/api/status"),
                                                              api("GET", "/api/sites"),
                                                              api("GET", "/api/presets"),
                                                              api("GET", "/api/session"),
  ]);
  state.status = status;
  state.sites = sites || [];
  state.presets = presets || [];
  state.session = session || { active: false };
  renderStatus();
  renderPresets();
  renderSites();
  renderSessionForm();
  renderGauge();
}

function renderStatus() {
  const s = state.status;
  if (!s) return;
  $("#status-os").textContent = s.os;
  $("#status-hosts").textContent = s.hosts_path;
  $("#hosts-path-inline").textContent = s.hosts_path;

  const elevatedEl = $("#status-elevated");
  elevatedEl.innerHTML = "";
  const pill = document.createElement("span");
  pill.className = "pill " + (s.elevated ? "good" : "bad");
  pill.textContent = s.elevated ? "permissions OK" : "no permissions";
  elevatedEl.appendChild(pill);

  $("#banner-elevation").style.display = s.elevated ? "none" : "block";
  $("#footer-meta").textContent = `focusguard v${s.version} · ${
    s.domains_blocked
  } domains currently blocked`;
}

function presetAlreadyAdded(key) {
  return state.sites.some((s) => s.preset === key);
}

function renderPresets() {
  const row = $("#preset-row");
  row.innerHTML = "";
  state.presets
  .filter((p) => !presetAlreadyAdded(p.key))
  .forEach((p) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "preset-chip";
    btn.textContent = "+ " + p.name;
    btn.addEventListener("click", async () => {
      btn.disabled = true;
      try {
        await api("POST", `/api/presets/${p.key}`, {});
        await loadAll();
      } catch (e) {
        showToast(e.message);
        btn.disabled = false;
      }
    });
    row.appendChild(btn);
  });
}

function isLocked(site) {
  const sess = state.session;
  if (!sess || !sess.active) return false;
  if (!sess.site_ids) return false;
  return sess.site_ids.includes(site.id);
}

function renderSites() {
  const list = $("#site-list");
  list.innerHTML = "";

  if (state.sites.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.textContent =
    "You haven't added any sites yet. Use the presets above or add one manually.";
    list.appendChild(empty);
    return;
  }

  state.sites.forEach((site) => {
    const locked = isLocked(site);
    const row = document.createElement("div");
    row.className =
    "site-row" +
    (site.enabled ? " is-blocked" : "") +
    (locked ? " is-locked" : "");

    const main = document.createElement("div");
    main.className = "site-main";
    const name = document.createElement("div");
    name.className = "site-name";
    name.innerHTML = `${escapeHtml(site.name)} <span class="lock-glyph">🔒 locked by session</span>`;
    const domains = document.createElement("div");
    domains.className = "site-domains";
    domains.textContent = site.domains.join(", ");
    main.appendChild(name);
    main.appendChild(domains);

    const controls = document.createElement("div");
    controls.className = "site-controls";

    const label = document.createElement("label");
    label.className = "switch";
    const input = document.createElement("input");
    input.type = "checkbox";
    input.checked = !!site.enabled;
    input.disabled = locked;
    input.addEventListener("change", async () => {
      const next = input.checked;
      input.disabled = true;
      try {
        await api("POST", `/api/sites/${site.id}/toggle`, { enabled: next });
        await loadAll();
      } catch (e) {
        showToast(e.message);
        await loadAll();
      }
    });
    const track = document.createElement("span");
    track.className = "track";
    label.appendChild(input);
    label.appendChild(track);

    const del = document.createElement("button");
    del.className = "icon-btn";
    del.title = "Delete";
    del.textContent = "✕";
    del.disabled = locked;
    del.addEventListener("click", async () => {
      if (locked) return;
      if (!confirm(`Delete "${site.name}" from the list?`)) return;
      try {
        await api("DELETE", `/api/sites/${site.id}`);
        await loadAll();
      } catch (e) {
        showToast(e.message);
      }
    });

    controls.appendChild(label);
    controls.appendChild(del);

    row.appendChild(main);
    row.appendChild(controls);
    list.appendChild(row);
  });
}

function renderSessionForm() {
  const form = $("#session-form");
  const checklist = $("#session-checklist");
  const startBtn = $("#session-start-btn");

  if (state.session && state.session.active) {
    form.style.display = "none";
    return;
  }
  form.style.display = "";

  checklist.innerHTML = "";
  if (state.sites.length === 0) {
    checklist.innerHTML =
    '<span style="color: var(--text-faint); font-size: 12.5px;">Add sites first.</span>';
    startBtn.disabled = true;
    return;
  }
  startBtn.disabled = false;

  state.sites.forEach((site) => {
    const lbl = document.createElement("label");
    const cb = document.createElement("input");
    cb.type = "checkbox";
    cb.value = site.id;
    cb.checked = !!site.enabled;
    lbl.appendChild(cb);
    lbl.appendChild(document.createTextNode(site.name));
    checklist.appendChild(lbl);
  });
}

function renderGauge() {
  const sess = state.session;
  const progress = $("#gauge-progress");
  const timeEl = $("#gauge-time");
  const labelEl = $("#gauge-label");
  const lockedWrap = $("#session-locked-list");
  const lockedNames = $("#session-locked-names");

  if (!sess || !sess.active) {
    progress.style.strokeDashoffset = CIRC;
    timeEl.textContent = "no session";
    timeEl.classList.add("idle");
    labelEl.textContent = "";
    lockedWrap.style.display = "none";
    clearInterval(renderGauge._timer);
    return;
  }

  timeEl.classList.remove("idle");
  labelEl.textContent = sess.label || "active focus";

  const names = state.sites
  .filter((s) => sess.site_ids && sess.site_ids.includes(s.id))
  .map((s) => s.name)
  .join(", ");
  lockedNames.textContent = names;
  lockedWrap.style.display = "block";

  const endsAt = new Date(sess.ends_at).getTime();

  function tick() {
    const remainMs = Math.max(0, endsAt - Date.now());
    const totalMin = Math.ceil(remainMs / 60000);
    const mm = Math.floor(remainMs / 60000);
    const ss = Math.floor((remainMs % 60000) / 1000);
    timeEl.textContent =
    String(mm).padStart(2, "0") + ":" + String(ss).padStart(2, "0");

    // reference: starts full and empties as time goes on
    const startMs = new Date(sess.ends_at).getTime() - (sess._totalMs || remainMs);
    const fraction = remainMs <= 0 ? 0 : remainMs / (sess._totalMs || remainMs);
    progress.style.strokeDashoffset = String(CIRC * (1 - fraction));

    if (remainMs <= 0) {
      clearInterval(renderGauge._timer);
      loadAll();
    }
  }

  if (!sess._totalMs) {
    sess._totalMs = Math.max(1, endsAt - Date.now());
  }

  clearInterval(renderGauge._timer);
  tick();
  renderGauge._timer = setInterval(tick, 1000);
}

function escapeHtml(str) {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

$("#add-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const form = e.target;
  const name = form.name.value.trim();
  const domains = form.domains.value
  .split(",")
  .map((d) => d.trim())
  .filter(Boolean);
  if (!name || domains.length === 0) {
    showToast("Please enter a name and at least one domain");
    return;
  }
  try {
    await api("POST", "/api/sites", { name, domains });
    form.reset();
    await loadAll();
  } catch (e2) {
    showToast(e2.message);
  }
});

$("#session-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const siteIds = Array.from(
    document.querySelectorAll("#session-checklist input:checked")
  ).map((cb) => cb.value);
  const minutes = parseInt($("#session-minutes").value, 10);
  if (siteIds.length === 0) {
    showToast("Select at least one site for the session");
    return;
  }
  try {
    await api("POST", "/api/session/start", {
      site_ids: siteIds,
      minutes,
      label: "focus",
    });
    await loadAll();
  } catch (e2) {
    showToast(e2.message);
  }
});

loadAll().catch((e) => showToast(e.message));
setInterval(() => loadAll().catch(() => {}), 15000);
