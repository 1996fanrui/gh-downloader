function detectPlatform() {
  const ua = navigator.userAgent.toLowerCase();
  const platform = (navigator.userAgentData?.platform || navigator.platform || '').toLowerCase();
  const arch = (navigator.userAgentData?.architecture || '').toLowerCase();
  if (ua.includes('mac') || platform.includes('mac')) return arch.includes('x86') ? 'mac/x64' : 'mac/arm64';
  if (ua.includes('windows') || platform.includes('win')) return arch.includes('arm') ? 'win/arm64' : 'win/x64';
  if (ua.includes('linux') || platform.includes('linux')) {
    return arch.includes('arm') || arch.includes('aarch') ? 'linux/arm64' : 'linux/x64';
  }
  return 'win/x64';
}

async function loadRepos() {
  const list = document.getElementById('repoList');
  list.innerHTML = '<p class="muted small" style="padding:24px 20px">正在加载工具列表…</p>';
  const summaries = await requestJSON(API.repos);
  clearRepos();
  const repos = await Promise.all(summaries.map(summary => requestJSON(API.repo(summary.owner, summary.name))));
  repos.forEach(replaceRepo);
  renderRepoList();
  routeFromLocation();
}

function downloadAsset(assetName) {
  const repo = REPOS[curRepo];
  const version = curVersion();
  window.location.href = API.download(repo.owner, repo.name, version.tag, assetName);
}

function downloadRecommended() {
  const entry = curVersion().map[effectivePlatform()];
  if (entry) downloadAsset(entry.asset);
}

async function syncRepo() {
  if (!curRepo) return;
  const repo = REPOS[curRepo];
  const button = document.getElementById('syncBtn');
  button.textContent = '↻ 同步中…';
  button.disabled = true;
  try {
    const updated = await requestJSON(API.sync(repo.owner, repo.name), { method: 'POST' });
    delete REPOS[curRepo];
    replaceRepo(updated);
    curRepo = repoKey(updated);
    curVerIdx = 0;
    document.getElementById('dSynced').textContent = `最后同步：${REPOS[curRepo].synced}`;
    buildVerSelects();
    renderAll();
    button.textContent = '✓ 已同步';
  } catch (err) {
    alert(err.message);
  } finally {
    setTimeout(() => {
      button.textContent = '检查更新';
      button.disabled = false;
    }, 1200);
  }
}

async function runAdd() {
  const value = document.getElementById('repoInput').value.trim();
  if (!value) return;
  startAddProgress();
  try {
    const repo = await requestJSON(API.repos, {
      method: 'POST',
      body: JSON.stringify({ repo: value })
    });
    finishAddProgress();
    replaceRepo(repo);
    setAddStatus('分析完成，正在打开仓库页面。');
    closeAdd();
    renderRepoList();
    openRepo(repoKey(repo));
  } catch (err) {
    setAddStatus(`添加失败：${err.message}`);
    resetAddButton();
  }
}

function startAddProgress() {
  document.getElementById('addProg').classList.add('on');
  const button = document.getElementById('addBtn');
  button.disabled = true;
  button.textContent = '分析中…';
  setAddStatus('正在拉取 Release 并分析平台下载包，大型仓库首次分析可能需要 1 分钟左右。');
  ['ps1', 'ps2', 'ps3', 'ps4'].forEach(id => {
    const el = document.getElementById(id);
    el.className = 'prog-step active';
    el.textContent = `◐ ${el.textContent.slice(2)}`;
  });
}

function finishAddProgress() {
  ['ps1', 'ps2', 'ps3', 'ps4'].forEach(id => {
    const el = document.getElementById(id);
    el.className = 'prog-step done';
    el.textContent = `✓ ${el.textContent.slice(2)}`;
  });
}

function setAddStatus(message) {
  document.getElementById('addStatus').textContent = message;
}

function resetAddButton() {
  const button = document.getElementById('addBtn');
  button.disabled = false;
  button.textContent = '添加';
}

detectedPlatform = detectPlatform();
curPlatform = detectedPlatform;
window.addEventListener('popstate', routeFromLocation);
loadRepos().catch(err => {
  clearRepos();
  document.getElementById('repoList').innerHTML =
    `<p class="muted small" style="padding:24px 20px">加载失败：${escapeHTML(err.message)}</p>`;
});
