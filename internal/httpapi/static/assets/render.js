function renderRepoList() {
  const query = (document.getElementById('searchBox').value || '').toLowerCase();
  const rows = Object.entries(REPOS)
    .filter(([key, repo]) => !query || key.toLowerCase().includes(query) || repo.desc.toLowerCase().includes(query))
    .map(renderRepoRow)
    .join('');
  document.getElementById('repoList').innerHTML =
    rows || '<p class="muted small" style="padding:24px 20px">还没有工具。点击右上角添加仓库。</p>';
}

function renderRepoRow([key, repo]) {
  const latest = repo.versions[0] || { tag: '未同步', map: {} };
  const families = ['win', 'mac', 'linux'].map(family =>
    Object.entries(latest.map || {}).some(([platform, entry]) => entry && platform.startsWith(family))
      ? `<i class="${family[0]}"></i>`
      : '<i class="n"></i>'
  ).join('');
  return `
    <button class="repo-row" onclick="openRepo('${escapeJS(key)}')">
      <span class="rail">${families}</span>
      <span style="flex:1; min-width:0">
        <span class="repo-name">${escapeHTML(key)}</span>
        <span class="repo-desc" style="display:block">${escapeHTML(repo.desc)}</span>
      </span>
      <span class="repo-right">
        <span class="tag">${escapeHTML(latest.tag)}</span>
        <span class="xs muted num">${repo.versions.length} 个版本</span>
      </span>
    </button>`;
}

function showList() {
  document.getElementById('page-list').classList.remove('hidden');
  document.getElementById('page-detail').classList.add('hidden');
  if (window.location.pathname !== '/') history.pushState(null, '', '/');
}

function openRepo(key) {
  openRepoView(key);
  const path = `/repos/${key.split('/').map(encodeURIComponent).join('/')}`;
  if (window.location.pathname !== path) history.pushState(null, '', path);
}

function openRepoView(key) {
  curRepo = key;
  curVerIdx = 0;
  curInstallMethodKey = '';
  const repo = REPOS[key];
  if (!repo || repo.versions.length === 0) return;
  document.getElementById('page-list').classList.add('hidden');
  document.getElementById('page-detail').classList.remove('hidden');
  document.getElementById('dRepoName').textContent = key;
  document.getElementById('dRepoLink').href = repo.htmlURL;
  document.getElementById('dDesc').textContent = repo.desc;
  document.getElementById('dSynced').textContent = `最后同步：${repo.synced}`;
  buildVerSelects();
  switchTab('B');
  renderAll();
  window.scrollTo(0, 0);
}

function routeFromLocation() {
  const match = window.location.pathname.match(/^\/repos\/([^/]+)\/([^/]+)\/?$/);
  if (!match) {
    showList();
    return;
  }
  const key = `${decodeURIComponent(match[1])}/${decodeURIComponent(match[2])}`;
  if (REPOS[key]) openRepoView(key);
  else showList();
}

function buildVerSelects() {
  const repo = REPOS[curRepo];
  const options = repo.versions.map((version, index) =>
    `<option value="${index}">${escapeHTML(version.tag)}${index === 0 ? '（最新）' : ''} · ${escapeHTML(version.date)}</option>`
  ).join('');
  ['verSelectB', 'verSelect'].forEach(id => {
    const el = document.getElementById(id);
    el.innerHTML = options;
    el.value = curVerIdx;
  });
}

function onVerChange(value) {
  curVerIdx = Number(value);
  curInstallMethodKey = '';
  buildVerSelects();
  renderAll();
}

function onPlatformChange(value) {
  curPlatform = value;
  curInstallMethodKey = '';
  renderB();
  renderAssets();
}

function renderAll() {
  document.getElementById('dVersion').textContent = curVersion().tag;
  document.getElementById('oldVerBanner').classList.toggle('hidden', isLatest());
  renderB();
  renderAssets();
}

function switchTab(tab) {
  document.getElementById('tabB').classList.toggle('on', tab === 'B');
  document.getElementById('tabA').classList.toggle('on', tab === 'A');
  document.getElementById('viewB').classList.toggle('hidden', tab !== 'B');
  document.getElementById('viewA').classList.toggle('hidden', tab !== 'A');
}

function renderB() {
  if (!curRepo) return;
  const repo = REPOS[curRepo];
  const version = curVersion();
  let platform = effectivePlatform();
  let entry = version.map[platform];
  let recommendedMethods = sortedRecommendedMethods(repo.installMethods, platform);
  if (!entry && recommendedMethods.length === 0) {
    const fallback = firstAvailablePlatform(version);
    if (fallback) {
      curPlatform = fallback;
      platform = fallback;
      entry = version.map[platform];
      recommendedMethods = sortedRecommendedMethods(repo.installMethods, platform);
    }
  }
  const platformLabel = PLATFORMS[platform];
  document.getElementById('warnNote')?.remove();
  if (!entry && recommendedMethods.length === 0) {
    renderMissingAsset(version, platform, platformLabel);
    return;
  }
  document.getElementById('bMain').classList.remove('hidden');
  document.getElementById('bNoAsset').classList.add('hidden');
  document.getElementById('detectDot').style.background = `var(--${platform.split('/')[0]})`;
  document.getElementById('platformMode').textContent = platform === detectedPlatform ? '当前系统' : '选择系统';
  renderPlatformSelect(version, platform, repo);
  if (entry) {
    document.getElementById('dlSize').textContent = `（${fmtSize(entry.size)}）`;
    document.getElementById('dlFile').textContent = entry.asset;
  }
  renderInstallSteps(entry, platform, recommendedMethods);
  renderVerify(entry);
}

function renderPlatformSelect(version, platform, repo) {
  const options = Object.entries(PLATFORMS).map(([key, item]) => {
    const available = version.map[key] || sortedRecommendedMethods(repo.installMethods, key).length > 0;
    const disabled = available ? '' : ' disabled';
    const suffix = [
      key === detectedPlatform ? '当前系统' : '',
      disabled ? '暂无' : ''
    ].filter(Boolean).join('，');
    return `<option value="${key}"${key === platform ? ' selected' : ''}${disabled}>${escapeHTML(item.name)}${suffix ? `（${escapeHTML(suffix)}）` : ''}</option>`;
  }).join('');
  document.getElementById('platformSelect').innerHTML = options;
}

function renderMissingAsset(version, platform, platformLabel) {
  document.getElementById('bMain').classList.add('hidden');
  document.getElementById('bNoAsset').classList.remove('hidden');
  document.getElementById('bNoAssetTitle').textContent = `${version.tag} 暂无 ${platformLabel.name} 版本`;
  const others = Object.entries(version.map).filter(([, entry]) => entry).map(([key]) => PLATFORMS[key].name);
  const fallback = REPOS[curRepo].versions.findIndex((item, index) => index !== curVerIdx && item.map[platform]);
  document.getElementById('bNoAssetDesc').innerHTML =
    (others.length ? `该版本提供：${[...new Set(others)].map(escapeHTML).join('、')}。` : '该版本未发布任何可安装产物。') +
    (fallback >= 0 ? ` <button class="linklike" onclick="onVerChange(${fallback})">${escapeHTML(REPOS[curRepo].versions[fallback].tag)} 有该平台版本 →</button>` : '');
}

function renderInstallSteps(entry, platform, recommendedMethods) {
  const kind = entry ? inferKind(entry.asset, entry.kind) : '';
  const repo = REPOS[curRepo];
  const downloadURL = entry
    ? new URL(API.download(repo.owner, repo.name, curVersion().tag, entry.asset), window.location.origin).toString()
    : '';
  const options = installOptions(repo, entry, platform, kind, downloadURL, recommendedMethods);
  if (!options.some(option => option.key === curInstallMethodKey)) curInstallMethodKey = options[0]?.key || '';
  const selected = options.find(option => option.key === curInstallMethodKey) || options[0];
  document.getElementById('installChoices').innerHTML = renderInstallChoices(options, selected);
  document.getElementById('assetDownloadPanel').classList.toggle('hidden', selected?.key !== 'asset');
  const steps = selected ? selected.steps : [];
  document.getElementById('installIntro').textContent = selected ? selected.intro : '';
  let html = steps.map(renderStep).join('');
  if (platform.startsWith('mac')) html += `<li>${MAC_NOTE}</li>`;
  document.getElementById('installSteps').innerHTML = html;
  if (entry?.warn) {
    document.getElementById('installSteps').insertAdjacentHTML(
      'beforebegin',
      `<div class="note" id="warnNote">⚠️ ${escapeHTML(entry.warn)}</div>`
    );
  }
}

function installOptions(repo, entry, platform, kind, downloadURL, recommendedMethods) {
  const official = recommendedMethods
    .map((method, index) => ({
      key: `official-${index}`,
      title: method.recommended ? `${method.title}（推荐）` : method.title,
      intro: '官方 README 推荐的通用安装方式，不绑定具体 release 版本。',
      steps: officialInstallSteps(method, platform)
    }));
  const usageCommand = recommendedMethods.find(method => method.usageCommand)?.usageCommand || '';
  if (!entry) return official;
  const assetSteps = appendUsageStep(installStepsFor(entry, platform, kind, downloadURL), usageCommand);
  return official.concat([{
    key: 'asset',
    title: '通过安装包安装',
    intro: installIntro(platform, kind, assetSteps, displayInstallCommand(entry.install)),
    steps: assetSteps
  }]);
}

function renderInstallChoices(options, selected) {
  if (options.length <= 1) return '';
  return options.map(option =>
    `<button class="chip ${option.key === selected.key ? 'on' : ''}" onclick="selectInstallMethod('${escapeJS(option.key)}')">${escapeHTML(option.title)}</button>`
  ).join('');
}

function selectInstallMethod(key) {
  curInstallMethodKey = key;
  renderB();
}

function installStepsFor(entry, platform, kind, downloadURL) {
  const documentedInstall = displayInstallCommand(entry.install);
  if (!hasDeterministicInstall(kind) && documentedInstall) return documentedInstallSteps(documentedInstall);
  return (TEMPLATES[kind] || TEMPLATES.download)(entry.asset, platform, downloadURL, binaryNameFor(entry));
}

function appendUsageStep(steps, usageCommand) {
  if (!usageCommand) return steps;
  return steps.concat([{ text: '安装后可运行', cmd: usageCommand }]);
}

function hasDeterministicInstall(kind) {
  return kind === 'installer' || kind === 'appimage' || kind === 'macapp';
}

function installIntro(platform, kind, steps, documentedInstall) {
  const hasCommand = steps.some(step => typeof step !== 'string' && step.cmd);
  if (!hasCommand) return '按下面步骤完成安装。';
  if (documentedInstall && !hasDeterministicInstall(kind)) return 'README 明确写出安装命令，可复制运行。';
  if (kind === 'appimage') return '在终端中复制脚本下载 AppImage；完成后手动打开应用。';
  if (kind === 'installer' || kind === 'macapp') {
    if (platform.startsWith('win')) return '在 PowerShell 中复制脚本下载安装器；完成后按安装器提示并手动打开应用。';
    if (platform.startsWith('mac')) return '在终端中复制脚本下载安装包；完成后从“应用程序”手动打开应用。';
  }
  if (platform.startsWith('win')) return '在 PowerShell 中复制并运行下面的脚本。';
  return '在终端中复制并运行下面的脚本。';
}

function binaryNameFor(entry) {
  const verify = displayVerifyCommand(entry.verify);
  const command = verify.split(/\s+/)[0];
  if (/^[A-Za-z0-9._-]+$/.test(command)) return command;
  return '';
}

function renderStep(step) {
  if (typeof step === 'string') return `<li>${step}</li>`;
  const id = `c${Math.random().toString(36).slice(2, 8)}`;
  return `<li>${step.text}
    <div class="cmd">
      <button class="copy-btn" onclick="copyCmd(this,'${id}')">复制</button>
      <pre><code id="${id}">${escapeHTML(step.cmd)}</code></pre>
    </div></li>`;
}

function renderVerify(entry) {
  const block = document.getElementById('verifyBlock');
  if (!entry) {
    block.classList.add('hidden');
    return;
  }
  const verify = displayVerifyCommand(entry.verify);
  if (!verify) {
    block.classList.add('hidden');
    return;
  }
  block.classList.remove('hidden');
  document.getElementById('verifyCmd').textContent = verify;
  document.getElementById('verifyIntro').textContent = 'README 明确写出可运行以下命令验证安装：';
}

function renderAssets() {
  if (!curRepo) return;
  const version = curVersion();
  const platform = effectivePlatform();
  const platformsOf = selectedPlatformsByAsset(version);
  const rows = sortedAssets(version).filter(asset => !(hideNoise && asset[2] === 'noise'));
  document.getElementById('assetBody').innerHTML = rows.map(asset => renderAssetRow(asset, platformsOf, platform)).join('');
  document.getElementById('assetCount').textContent =
    `${version.tag}：显示 ${rows.length} / ${version.assets.length} 个文件（该版本共 ${version.assetTotal} 个）。`;
}

function selectedPlatformsByAsset(version) {
  const result = {};
  for (const [platform, entry] of Object.entries(version.map)) {
    if (!entry) continue;
    (result[entry.asset] ||= []).push(platform);
  }
  return result;
}

function sortedAssets(version) {
  const byName = Object.fromEntries(version.assets.map(asset => [asset[0], asset]));
  const promoted = [];
  const promotedNames = new Set();
  for (const platform of Object.keys(PLATFORMS)) {
    const entry = version.map[platform];
    if (!entry || !byName[entry.asset] || promotedNames.has(entry.asset)) continue;
    promoted.push(byName[entry.asset]);
    promotedNames.add(entry.asset);
  }
  return promoted.concat(version.assets.filter(asset => !promotedNames.has(asset[0])));
}

function renderAssetRow([name, size], platformsOf, currentPlatform) {
  const platforms = platformsOf[name] || [];
  const isMine = platforms.includes(currentPlatform);
  const chips = platforms.map(platform =>
    `<span class="plat ${platform.split('/')[0]}${platform === currentPlatform ? ' me' : ''}">${PLATFORMS[platform].short}</span>`
  ).join('');
  return `<tr class="${isMine ? 'recommended' : ''}">
    <td><span class="fname">${escapeHTML(name)}</span></td>
    <td><div class="plat-cell">${chips}${isMine ? '<span class="tag tag-rec">推荐</span>' : ''}</div></td>
    <td class="size">${fmtSize(size)}</td>
    <td style="text-align:right"><button class="btn btn-sm ${isMine ? 'btn-primary' : ''}" onclick="downloadAsset('${escapeJS(name)}')">下载</button></td>
  </tr>`;
}

function toggleNoise() {
  hideNoise = !hideNoise;
  document.getElementById('chipNoise').classList.toggle('on', hideNoise);
  renderAssets();
}

function copyCmd(btn, id) {
  const text = document.getElementById(id).textContent;
  navigator.clipboard?.writeText(text);
  const original = btn.textContent;
  btn.textContent = '已复制';
  setTimeout(() => { btn.textContent = original; }, 1400);
}

function openAdd() {
  document.getElementById('addOverlay').classList.add('on');
}

function closeAdd() {
  document.getElementById('addOverlay').classList.remove('on');
  document.getElementById('addProg').classList.remove('on');
  document.getElementById('repoInput').value = '';
  document.getElementById('addStatus').textContent = '';
  ['ps1', 'ps2', 'ps3', 'ps4'].forEach(id => {
    const el = document.getElementById(id);
    el.className = 'prog-step';
    el.textContent = `○ ${el.textContent.slice(2)}`;
  });
  resetAddButton();
}
