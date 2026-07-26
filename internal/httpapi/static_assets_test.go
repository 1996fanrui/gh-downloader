package httpapi

import (
	"strings"
	"testing"
)

func TestAddRepoFlowShowsVisibleProgress(t *testing.T) {
	source := readStaticForTest(t, "static/index.html") + readStaticForTest(t, "static/assets/app.js")
	for _, want := range []string{
		`id="addStatus"`,
		"\u5206\u6790\u4e2d\u2026",
		"\u5927\u578b\u4ed3\u5e93\u9996\u6b21\u5206\u6790\u53ef\u80fd\u9700\u8981 1 \u5206\u949f\u5de6\u53f3",
		"\u6dfb\u52a0\u5931\u8d25\uff1a",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("add repo UI missing progress text %q", want)
		}
	}
}

func TestStaticUIContainsGuidedDownloadAndIconAssets(t *testing.T) {
	source := readStaticForTest(t, "static/index.html") + readStaticForTest(t, "static/assets/style.css")
	for _, want := range []string{
		`rel="icon" type="image/png" href="/assets/icon.png?v=issue-1-repo-url-routing"`,
		`class="brand-icon" src="/assets/icon.png?v=issue-1-repo-url-routing"`,
		`src="/assets/state.js?v=issue-1-repo-url-routing"`,
		`href="https://github.com/1996fanrui/gh-downloader"`,
		`class="btn btn-sm project-link"`,
		`class="project-link-icon"`,
		"\u9879\u76ee GitHub</span>",
		`id="tabB" onclick="switchTab('B')">` + "\u4e00\u952e\u4e0b\u8f7d",
		`.brand-icon`,
		`.project-link-icon`,
		`logo-button" onclick="showList()"`,
		"\u641c\u7d22\u5df2\u6536\u5f55\u5de5\u5177\u2026",
		">\u5173\u95ed</button>",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("static UI missing %q", want)
		}
	}
}

func TestStaticUIContainsInstallScripts(t *testing.T) {
	source := readStaticForTest(t, "static/index.html") +
		readStaticForTest(t, "static/assets/state.js") +
		readStaticForTest(t, "static/assets/render.js") +
		readStaticForTest(t, "static/assets/components.css")
	for _, want := range []string{
		`curl -L -o`,
		`$u = `,
		`Invoke-WebRequest -Uri $u -OutFile $f`,
		`new URL(API.download`,
		`id="installIntro"`,
		`id="platformSelect"`,
		`id="installChoices"`,
		`id="assetDownloadPanel"`,
		`id="dRepoLink" target="_blank" rel="noopener noreferrer"`,
		`document.getElementById('dRepoLink').href = repo.htmlURL`,
		`officialInstallSteps(method, platform)`,
		`<pre><code id="`,
		`top: 8px;`,
		`right: 8px;`,
		"\u4e0b\u8f7d AppImage \u5e76\u6dfb\u52a0\u6267\u884c\u6743\u9650",
		"\u786e\u8ba4\u5df2\u5b89\u88c5 npm",
		"README \u660e\u786e\u5199\u51fa\u5b89\u88c5\u547d\u4ee4",
		"README \u672a\u660e\u786e\u5199\u51fa\u5b89\u88c5\u547d\u4ee4",
		`displayVerifyCommand(entry.verify)`,
		`usageCommand`,
		`routeFromLocation()`,
		`history.pushState`,
		`/repos/${key.split('/')`,
		"\u5b89\u88c5\u540e\u53ef\u8fd0\u884c",
		"\u5f53\u524d\u7cfb\u7edf",
		"\u901a\u8fc7\u5b89\u88c5\u5305\u5b89\u88c5",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("static install UI missing %q", want)
		}
	}
}

func TestStaticUINeverFallsBackToVersionVerify(t *testing.T) {
	source := readStaticForTest(t, "static/assets/state.js") +
		readStaticForTest(t, "static/assets/render.js")
	for _, forbidden := range []string{
		"` --version`",
		"${value} --version",
		"\u901a\u5e38\u53ef\u4ee5\u8fd0\u884c\u4ee5\u4e0b\u547d\u4ee4\u786e\u8ba4\u5b89\u88c5\u6210\u529f",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("static UI still has verify fallback %q", forbidden)
		}
	}
}

func TestStaticUINeverInstallsArchivesWithoutReadmeCommand(t *testing.T) {
	source := readStaticForTest(t, "static/assets/state.js")
	for _, forbidden := range []string{
		"find . -type f -name",
		"\u4e0b\u8f7d\u3001\u89e3\u538b\u5e76\u5b89\u88c5\u5230 PATH",
		"Get-ChildItem -Path $dir -Recurse -File -Filter '*.exe'",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("static UI still auto-installs ambiguous archive %q", forbidden)
		}
	}
}

func TestStaticUIUsesChineseRepositoryDescriptions(t *testing.T) {
	source := readStaticForTest(t, "static/assets/api.js")
	for _, want := range []string{
		"\u7684\u5f00\u6e90\u5de5\u5177\u5b89\u88c5\u5305",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("static repository description missing %q", want)
		}
	}
	for _, forbidden := range []string{
		`const known`,
		`known[key]`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("static repository description has hardcoded content lookup %q", forbidden)
		}
	}
}

func readStaticForTest(t *testing.T, path string) string {
	t.Helper()
	data, err := staticFiles.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
