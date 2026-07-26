const API = {
  repos: '/api/repos',
  repo: (owner, name) => `/api/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}`,
  sync: (owner, name) => `/api/repos/${encodeURIComponent(owner)}/${encodeURIComponent(name)}/sync`,
  download: (owner, name, tag, asset) =>
    `/dl/${encodeURIComponent(owner)}/${encodeURIComponent(name)}/${encodeURIComponent(tag)}/${encodeURIComponent(asset)}`
};

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, ch => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[ch]));
}

function escapeJS(value) {
  return String(value ?? '').replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}

function repoKey(repo) {
  return repo.full_name || `${repo.owner}/${repo.name}`;
}

function displayDate(value) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value).slice(0, 10);
  return date.toISOString().slice(0, 10);
}

function displaySynced(value) {
  if (!value) return '尚未同步';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
}

function displayDescription(repo) {
  const value = String(repo.description || '').trim();
  if (/[\u4e00-\u9fff]/.test(value)) return value.length > 72 ? `${value.slice(0, 72)}…` : value;
  return `${repo.name} 的开源工具安装包。`;
}

function isNoiseAsset(asset) {
  const name = asset.name.toLowerCase();
  return name.includes('sha256') ||
    name.includes('checksum') ||
    name.includes('sigstore') ||
    name.endsWith('.sig') ||
    name.endsWith('.asc') ||
    name.endsWith('.blockmap') ||
    name.endsWith('.yml') ||
    name.endsWith('.yaml') ||
    name.includes('manifest') ||
    name.includes('d.sym') ||
    name.includes('dsym');
}

function normalizeSelection(selection) {
  if (!selection) return null;
  return {
    asset: selection.asset,
    size: 0,
    kind: selection.kind || null,
    install: selection.install || null,
    isrc: selection.install_source || null,
    verify: selection.verify || null,
    vsrc: selection.verify_source || null,
    warn: selection.note || null
  };
}

function normalizeRepo(repo) {
  return {
    owner: repo.owner,
    name: repo.name,
    htmlURL: repo.html_url || `https://github.com/${repo.owner}/${repo.name}`,
    desc: displayDescription(repo),
    installMethods: (repo.install_methods || []).map(normalizeInstallMethod),
    synced: displaySynced(repo.last_synced_at),
    versions: (repo.versions || []).map(normalizeVersion)
  };
}

function normalizeInstallMethod(method) {
  return {
    title: String(method.title || '官方推荐安装').trim(),
    manager: String(method.manager || 'other').trim(),
    command: String(method.command || '').trim(),
    usageCommand: method.usage_command ? String(method.usage_command).trim() : '',
    platforms: method.platforms || [],
    source: method.source || null,
    recommended: Boolean(method.recommended)
  };
}

function normalizeVersion(version) {
  const assets = version.assets || [];
  const byName = Object.fromEntries(assets.map(asset => [asset.name, asset]));
  const map = {};
  for (const key of Object.keys(PLATFORMS)) {
    const selection = normalizeSelection((version.platform_map || {})[key]);
    if (selection && byName[selection.asset]) selection.size = byName[selection.asset].size;
    map[key] = selection;
  }
  return {
    tag: version.tag,
    date: displayDate(version.published_at),
    assetTotal: version.asset_total || assets.length,
    map,
    assets: assets.map(asset => [asset.name, asset.size, isNoiseAsset(asset) ? 'noise' : ''])
  };
}

function replaceRepo(repo) {
  REPOS[repoKey(repo)] = normalizeRepo(repo);
}

function clearRepos() {
  for (const key of Object.keys(REPOS)) delete REPOS[key];
}

async function requestJSON(url, options) {
  const resp = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...options
  });
  const text = await resp.text();
  const data = text ? JSON.parse(text) : null;
  if (!resp.ok) throw new Error((data && data.error) || `${resp.status} ${resp.statusText}`);
  return data;
}
