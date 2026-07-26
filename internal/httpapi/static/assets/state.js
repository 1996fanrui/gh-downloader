const REPOS = {};

const PLATFORMS = {
  'win/x64': { name: 'Windows (64 位)', short: 'Win x64' },
  'win/arm64': { name: 'Windows (ARM64)', short: 'Win ARM' },
  'mac/arm64': { name: 'macOS (Apple 芯片)', short: 'Mac Apple' },
  'mac/x64': { name: 'macOS (Intel)', short: 'Mac Intel' },
  'linux/x64': { name: 'Linux (64 位)', short: 'Linux x64' },
  'linux/arm64': { name: 'Linux (ARM64)', short: 'Linux ARM' }
};

const EXT_KIND = {
  '.exe': 'installer',
  '.msi': 'installer',
  '.dmg': 'installer',
  '.pkg': 'installer',
  '.deb': 'installer',
  '.rpm': 'installer',
  '.appimage': 'appimage',
  '.apk': 'android',
  '.tar.gz': 'archive',
  '.tgz': 'archive',
  '.tar.xz': 'archive',
  '.tar.zst': 'archive',
  '.zip': 'archive'
};

// Installation rendering has two phases:
// Phase 1 selects the best asset for each supported platform.
// Phase 2 renders install guidance for each selected asset with four cases:
// 1. Deterministic auto-install: the format has a standard install path, so code
//    generates the install command. Examples: Linux .deb via apt, Linux .rpm via
//    rpm, or a documented CLI binary with a known command name.
// 2. Deterministic manual-open: the format clearly means users should download
//    and open it manually. Examples: Windows .exe/.msi, macOS .dmg/.pkg, and
//    Linux AppImage after chmod +x.
// 3. Documentation-backed install: the format is ambiguous, but AI extracted
//    explicit README install commands for the selected asset. Examples: a .zip
//    that documents which binary to move into PATH, or a .tar.gz that documents
//    the command name and post-install verification command.
// 4. Unknown install: neither deterministic rules nor README evidence can
//    identify a safe install flow. Examples: archives with multiple binaries,
//    unknown extensions, or a standalone binary without a documented command
//    name. In this case only render download/extract actions and tell users to
//    follow the project documentation.
const TEMPLATES = {
  binary: (filename, platform, downloadURL, binaryName) => binarySteps(filename, platform, downloadURL, binaryName),
  installer: (filename, platform, downloadURL, binaryName) => {
    if (platform.startsWith('linux')) return linuxInstallerSteps(filename, downloadURL);
    if (platform.startsWith('win')) return windowsInstallerSteps(filename, downloadURL);
    if (platform.startsWith('mac')) return macInstallerSteps(filename, downloadURL);
    const steps = [`下载完成后，双击 <code>${filename}</code>`, '按照安装向导提示完成安装'];
    return steps;
  },
  macapp: (filename, platform, downloadURL) => macInstallerSteps(filename, downloadURL),
  appimage: (filename, platform, downloadURL) => platform.startsWith('linux')
    ? [{
      text: '下载 AppImage 并添加执行权限',
      cmd: shellScript(
        'mkdir -p "$HOME/Downloads"',
        `target="$HOME/Downloads/${filename}"`,
        `curl -L -o "$target" ${shellQuote(downloadURL)}`,
        'chmod +x "$target"'
      )
    }, '下载完成后，从文件管理器手动打开 AppImage。']
    : [
      `下载 <code>${filename}</code>`,
      { text: '添加可执行权限', cmd: `chmod +x ${shellQuote(filename)}` },
      { text: '双击运行，或在终端执行', cmd: `./${filename}` }
    ],
  archive: (filename, platform, downloadURL) => {
    const isWindows = platform.startsWith('win');
    if (isWindows) return windowsArchiveSteps(filename, downloadURL);
    return unixArchiveDownloadSteps(filename, downloadURL);
  },
  download: (filename, platform, downloadURL) => downloadSteps(filename, platform, downloadURL),
  android: filename => [`下载 <code>${filename}</code>`, '在手机上打开该文件并允许「安装未知来源应用」']
};

const MAC_NOTE = '首次打开若提示「无法验证开发者」，请到 系统设置 → 隐私与安全性 中点击「仍要打开」。';

let curRepo = null;
let detectedPlatform = 'win/x64';
let curPlatform = 'win/x64';
let hideNoise = true;
let curVerIdx = 0;
let curInstallMethodKey = '';

function curVersion() {
  return REPOS[curRepo].versions[curVerIdx];
}

function isLatest() {
  return curVerIdx === 0;
}

function effectivePlatform() {
  return curPlatform;
}

function firstAvailablePlatform(version) {
  return Object.keys(PLATFORMS).find(platform => version.map[platform]);
}

function fmtSize(bytes) {
  if (bytes >= 1073741824) return `${(bytes / 1073741824).toFixed(1)} GB`;
  if (bytes >= 1048576) return `${(bytes / 1048576).toFixed(1)} MB`;
  return `${(bytes / 1024).toFixed(0)} KB`;
}

function inferKind(filename, aiKind) {
  if (aiKind) return aiKind;
  const lower = filename.toLowerCase();
  for (const [ext, kind] of Object.entries(EXT_KIND)) {
    if (lower.endsWith(ext)) return kind;
  }
  return 'download';
}

function shellQuote(value) {
  return `'${String(value).replace(/'/g, `'\\''`)}'`;
}

function powershellQuote(value) {
  return `'${String(value).replace(/'/g, `''`)}'`;
}

function shellScript(...commands) {
  return ['set -e', ...commands].join('\n');
}

function extractCommand(filename) {
  const quoted = shellQuote(filename);
  const lower = filename.toLowerCase();
  if (lower.endsWith('.tar.gz') || lower.endsWith('.tgz')) return `tar -xzf ${quoted}`;
  return `tar -xf ${quoted}`;
}

function windowsInstallerSteps(filename, downloadURL) {
  const quotedName = powershellQuote(filename);
  const quotedURL = powershellQuote(downloadURL);
  const lower = filename.toLowerCase();
  if (lower.endsWith('.msi')) {
    return [{
      text: '下载并启动安装器',
      cmd: [
        `$u = ${quotedURL}`,
        `$f = Join-Path $env:TEMP ${quotedName}`,
        'Invoke-WebRequest -Uri $u -OutFile $f',
        "Start-Process msiexec.exe -Wait -ArgumentList '/i', $f"
      ].join('\n')
    }];
  }
  return [{
    text: '下载并启动安装器',
    cmd: [
      `$u = ${quotedURL}`,
      `$f = Join-Path $env:TEMP ${quotedName}`,
      'Invoke-WebRequest -Uri $u -OutFile $f',
      'Start-Process -Wait $f'
    ].join('\n')
  }];
}

function binarySteps(filename, platform, downloadURL, binaryName) {
  if (platform.startsWith('win')) return windowsBinarySteps(filename, downloadURL, binaryName);
  if (platform.startsWith('linux') || platform.startsWith('mac')) {
    if (!binaryName) return unixBinaryDownloadSteps(filename, downloadURL);
    return [{
      text: '下载并安装到 PATH',
      cmd: shellScript(
        `curl -L -o ${shellQuote(binaryName)} ${shellQuote(downloadURL)}`,
        `chmod +x ${shellQuote(binaryName)}`,
        `sudo install -m 755 ${shellQuote(binaryName)} ${shellQuote(`/usr/local/bin/${binaryName}`)}`
      )
    }];
  }
  return [{ text: '下载安装包', cmd: `curl -L -o ${shellQuote(filename)} ${shellQuote(downloadURL)}` }];
}

function windowsBinarySteps(filename, downloadURL, binaryName) {
  const quotedName = powershellQuote(filename);
  const quotedURL = powershellQuote(downloadURL);
  if (!binaryName) {
    return [{
      text: '下载到用户程序目录',
      cmd: [
        `$u = ${quotedURL}`,
        `$name = ${quotedName}`,
        "$dir = Join-Path $env:LOCALAPPDATA 'Programs\\GithubDownloader'",
        'New-Item -ItemType Directory -Force -Path $dir | Out-Null',
        '$target = Join-Path $dir $name',
        'Invoke-WebRequest -Uri $u -OutFile $target'
      ].join('\n')
    }, 'README 未明确写出命令名，请按项目文档手动启动或加入 PATH。'];
  }
  const quotedBinary = powershellQuote(binaryName);
  return [{
    text: '下载并加入用户 PATH',
    cmd: [
      `$u = ${quotedURL}`,
      `$name = ${quotedName}`,
      `$app = ${quotedBinary}`,
      "$dir = Join-Path $env:LOCALAPPDATA ('Programs\\' + $app)",
      'New-Item -ItemType Directory -Force -Path $dir | Out-Null',
      '$target = Join-Path $dir $name',
      'Invoke-WebRequest -Uri $u -OutFile $target',
      "$path = [Environment]::GetEnvironmentVariable('Path', 'User')",
      "if (($path -split ';') -notcontains $dir) { [Environment]::SetEnvironmentVariable('Path', ($path.TrimEnd(';') + ';' + $dir), 'User') }"
    ].join('\n')
  }];
}

function windowsArchiveSteps(filename, downloadURL) {
  const quotedName = powershellQuote(filename);
  const quotedURL = powershellQuote(downloadURL);
  return [{
    text: '下载并解压',
    cmd: [
      `$u = ${quotedURL}`,
      `$name = ${quotedName}`,
      "$dir = Join-Path $env:LOCALAPPDATA 'Programs\\GithubDownloader'",
      'New-Item -ItemType Directory -Force -Path $dir | Out-Null',
      '$f = Join-Path $env:TEMP $name',
      'Invoke-WebRequest -Uri $u -OutFile $f',
      "if ($name.ToLower().EndsWith('.zip')) { Expand-Archive -Force $f $dir } else { tar -xf $f -C $dir }",
      'Start-Process $dir'
    ].join('\n')
  }, 'README 未明确写出安装命令，请按项目文档继续。'];
}

function macInstallerSteps(filename, downloadURL) {
  return [
    {
      text: '下载并打开安装包',
      cmd: shellScript(
        'mkdir -p "$HOME/Downloads"',
        `target="$HOME/Downloads/${filename}"`,
        `curl -L -o "$target" ${shellQuote(downloadURL)}`,
        'open "$target"'
      )
    },
    '按安装包窗口提示完成安装。',
    '安装完成后，从“应用程序”手动打开应用。'
  ];
}

function linuxInstallerSteps(filename, downloadURL) {
  const quotedName = shellQuote(filename);
  const quotedURL = shellQuote(downloadURL);
  const lower = filename.toLowerCase();
  if (lower.endsWith('.deb')) {
    return [{
      text: '下载并安装',
      cmd: shellScript(
        `curl -L -o ${quotedName} ${quotedURL}`,
        `sudo apt install -y ./${quotedName}`
      )
    }];
  }
  if (lower.endsWith('.rpm')) {
    return [{
      text: '下载并安装',
      cmd: shellScript(
        `curl -L -o ${quotedName} ${quotedURL}`,
        `sudo rpm -i ${quotedName}`
      )
    }];
  }
  return [{ text: '下载安装包', cmd: `curl -L -o ${quotedName} ${quotedURL}` }];
}

function unixArchiveDownloadSteps(filename, downloadURL) {
  return [{
    text: '下载并解压',
    cmd: shellScript(
      `curl -L -o ${shellQuote(filename)} ${shellQuote(downloadURL)}`,
      extractCommand(filename)
    )
  }, 'README 未明确写出安装命令，请按项目文档继续。'];
}

function documentedInstallSteps(command) {
  return [{
    text: '按 README 说明安装',
    cmd: command
  }];
}

function unixBinaryDownloadSteps(filename, downloadURL) {
  return [{
    text: '下载并添加执行权限',
    cmd: shellScript(
      `curl -L -o ${shellQuote(filename)} ${shellQuote(downloadURL)}`,
      `chmod +x ${shellQuote(filename)}`
    )
  }, 'README 未明确写出命令名，请按项目文档手动启动或加入 PATH。'];
}

function downloadSteps(filename, platform, downloadURL) {
  if (platform.startsWith('win')) {
    const quotedName = powershellQuote(filename);
    const quotedURL = powershellQuote(downloadURL);
    return [{
      text: '下载文件',
      cmd: [
        `$u = ${quotedURL}`,
        `$f = Join-Path $env:TEMP ${quotedName}`,
        'Invoke-WebRequest -Uri $u -OutFile $f',
        'Start-Process (Split-Path $f)'
      ].join('\n')
    }, '无法仅通过文件名确定安装方式，请按项目文档继续。'];
  }
  return [{
    text: '下载文件',
    cmd: shellScript(`curl -L -o ${shellQuote(filename)} ${shellQuote(downloadURL)}`)
  }, '无法仅通过文件名确定安装方式，请按项目文档继续。'];
}

function displayVerifyCommand(command) {
  const value = String(command || '').trim();
  if (/^[A-Za-z0-9._/-]+$/.test(value)) return '';
  return value;
}

function displayInstallCommand(command) {
  const value = String(command || '').trim();
  if (/^[A-Za-z0-9._/-]+$/.test(value)) return '';
  return value;
}

function methodAppliesToPlatform(method, platform) {
  return !method.platforms || method.platforms.length === 0 || method.platforms.includes(platform);
}

const MANAGER_PRIORITY = {
  npm: 10,
  homebrew: 20,
  cargo: 30,
  pipx: 40,
  winget: 50,
  scoop: 60,
  choco: 70,
  apt: 80,
  dnf: 90,
  yum: 100,
  pacman: 110,
  zypper: 120,
  apk: 130,
  mise: 140,
  nix: 150,
  guix: 160,
  macports: 170,
  remote_script: 900,
  other: 1000
};

function sortedRecommendedMethods(methods, platform) {
  const applicable = (methods || [])
    .filter(method => method.command && methodAppliesToPlatform(method, platform));
  const visible = applicable.some(method => method.manager !== 'remote_script')
    ? applicable.filter(method => method.manager !== 'remote_script')
    : applicable;
  return visible
    .slice()
    .sort((a, b) => {
      const pa = MANAGER_PRIORITY[a.manager] || MANAGER_PRIORITY.other;
      const pb = MANAGER_PRIORITY[b.manager] || MANAGER_PRIORITY.other;
      if (pa !== pb) return pa - pb;
      if (a.recommended !== b.recommended) return a.recommended ? -1 : 1;
      return a.title.localeCompare(b.title, 'zh-CN');
    })
    .slice(0, 2);
}

function usesNPM(command) {
  return /(^|\n)\s*(npm|npx)\s+/i.test(command);
}

function npmPrereqStep(platform) {
  if (platform.startsWith('win')) {
    return {
      text: '确认已安装 npm',
      cmd: [
        'if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {',
        "  throw '未检测到 npm，请先安装 Node.js LTS：https://nodejs.org'",
        '}'
      ].join('\n')
    };
  }
  return {
    text: '确认已安装 npm',
    cmd: shellScript(
      'command -v npm >/dev/null 2>&1 || {',
      "  echo '未检测到 npm，请先安装 Node.js LTS：https://nodejs.org' >&2",
      '  exit 1',
      '}'
    )
  };
}

function officialInstallSteps(method, platform) {
  const steps = [];
  if (usesNPM(method.command)) steps.push(npmPrereqStep(platform));
  steps.push({ text: method.title || '官方推荐安装', cmd: method.command });
  if (method.usageCommand) steps.push({ text: '安装后可运行', cmd: method.usageCommand });
  return steps;
}
