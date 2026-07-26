package analyzer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/1996fanrui/gh-downloader/internal/domain"
)

func TestDescribeGeneratesChineseDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"description\":\"\u5feb\u901f\u6613\u7528\u7684\u6587\u4ef6\u67e5\u627e\u5de5\u5177\u3002\"}"}}]}`))
	}))
	defer server.Close()

	analyzer := NewOpenAIAnalyzer("key", server.URL, "model", server.Client())
	got, err := analyzer.Describe(context.Background(), DescriptionInput{
		RepoFullName:      "owner/repo",
		GitHubDescription: "A fast command-line tool.",
		Readme:            "README",
	})
	if err != nil {
		t.Fatalf("Describe returned error: %v", err)
	}
	want := "\u5feb\u901f\u6613\u7528\u7684\u6587\u4ef6\u67e5\u627e\u5de5\u5177\u3002"
	if got != want {
		t.Fatalf("Describe = %q, want %q", got, want)
	}
}

func TestDescriptionPromptRequiresSimplifiedChinese(t *testing.T) {
	if !strings.Contains(descriptionSystemPrompt(), "Simplified Chinese") {
		t.Fatal("description prompt does not require Simplified Chinese")
	}
	if err := validateDescription("A fast command-line tool."); err == nil {
		t.Fatal("validateDescription accepted an English description")
	}
}

func TestPromptsIncludeFullReadme(t *testing.T) {
	readme := strings.Repeat("a", 13000) + "VERIFY_AT_END"

	analysisPrompt := userPrompt(Input{RepoFullName: "owner/repo", Tag: "v1", Readme: readme})
	if !strings.Contains(analysisPrompt, "VERIFY_AT_END") {
		t.Fatal("analysis prompt truncated README")
	}

	descriptionPrompt := descriptionUserPrompt(DescriptionInput{RepoFullName: "owner/repo", Readme: readme})
	if !strings.Contains(descriptionPrompt, "VERIFY_AT_END") {
		t.Fatal("description prompt truncated README")
	}

	installPrompt := installMethodsUserPrompt(InstallMethodsInput{RepoFullName: "owner/repo", Readme: readme})
	if !strings.Contains(installPrompt, "VERIFY_AT_END") {
		t.Fatal("install methods prompt truncated README")
	}
}

func TestNormalizeInstallMethodsPreservesUsageCommand(t *testing.T) {
	usageCommand := "codex"

	got := normalizeInstallMethods([]installMethodJSON{{
		Title:        "npm install",
		Manager:      "npm",
		Command:      "npm install -g @openai/codex",
		UsageCommand: &usageCommand,
		Platforms:    []string{"win/x64", "linux/x64"},
		Source:       "readme",
		Recommended:  true,
	}})

	if len(got) != 1 {
		t.Fatalf("len(methods) = %d, want 1", len(got))
	}
	if got[0].UsageCommand == nil || *got[0].UsageCommand != "codex" {
		t.Fatalf("UsageCommand = %v, want codex", got[0].UsageCommand)
	}
	if got[0].Manager != "npm" {
		t.Fatalf("Manager = %q, want npm", got[0].Manager)
	}
	if len(got[0].Platforms) != 2 {
		t.Fatalf("Platforms = %v, want two platforms", got[0].Platforms)
	}
}

func TestNormalizeInstallMethodsKeepsRemoteScripts(t *testing.T) {
	got := normalizeInstallMethods([]installMethodJSON{{
		Title:       "installer",
		Manager:     "remote_script",
		Command:     "curl -fsSL https://example.test/install.sh | sh",
		Platforms:   []string{"linux/x64"},
		Source:      "readme",
		Recommended: true,
	}})

	if len(got) != 1 {
		t.Fatalf("len(methods) = %d, want 1", len(got))
	}
	if got[0].Manager != "remote_script" {
		t.Fatalf("Manager = %q, want remote_script", got[0].Manager)
	}
}

func TestNormalizeInstallMethodsClassifiesRemoteScripts(t *testing.T) {
	got := normalizeInstallMethods([]installMethodJSON{{
		Title:       "installer",
		Manager:     "other",
		Command:     "iex (irm https://example.test/install.ps1)",
		Platforms:   []string{"win/x64"},
		Source:      "readme",
		Recommended: true,
	}})

	if len(got) != 1 {
		t.Fatalf("len(methods) = %d, want 1", len(got))
	}
	if got[0].Manager != "remote_script" {
		t.Fatalf("Manager = %q, want remote_script", got[0].Manager)
	}
}

func TestNormalizeInstallMethodsDropsDevelopmentInstalls(t *testing.T) {
	got := normalizeInstallMethods([]installMethodJSON{
		{
			Title:       "Development dependencies",
			Manager:     "npm",
			Command:     "npm install --ignore-scripts",
			Platforms:   []string{"linux/x64"},
			Source:      "readme",
			Recommended: true,
		},
		{
			Title:       "Editable development install",
			Manager:     "other",
			Command:     `uv pip install -e ".[all,dev]"`,
			Platforms:   []string{"linux/x64"},
			Source:      "readme",
			Recommended: true,
		},
	})

	if len(got) != 0 {
		t.Fatalf("len(methods) = %d, want 0", len(got))
	}
}

func TestNormalizeInstallMethodsKeepsEndUserPackageInstall(t *testing.T) {
	got := normalizeInstallMethods([]installMethodJSON{{
		Title:       "npm install",
		Manager:     "npm",
		Command:     "npm install -g @openai/codex",
		Platforms:   []string{"linux/x64"},
		Source:      "readme",
		Recommended: true,
	}})

	if len(got) != 1 {
		t.Fatalf("len(methods) = %d, want 1", len(got))
	}
	if got[0].Command != "npm install -g @openai/codex" {
		t.Fatalf("Command = %q", got[0].Command)
	}
}

func TestNormalizeInstallMethodsDropsManagerMismatch(t *testing.T) {
	got := normalizeInstallMethods([]installMethodJSON{{
		Title:       "npm install",
		Manager:     "npm",
		Command:     "brew install codex",
		Platforms:   []string{"mac/arm64"},
		Source:      "readme",
		Recommended: true,
	}})

	if len(got) != 0 {
		t.Fatalf("len(methods) = %d, want 0", len(got))
	}
}

func TestSelectionToDomainDropsBareVerifyCommand(t *testing.T) {
	verify := "codex"
	source := "readme"

	got := selectionToDomain(&selectionJSON{
		Asset:        "codex-x86_64-unknown-linux-musl.tar.gz",
		Verify:       &verify,
		VerifySource: &source,
	})

	if got.Verify != nil {
		t.Fatalf("Verify = %q, want nil", *got.Verify)
	}
	if got.VerifySource != nil {
		t.Fatalf("VerifySource = %q, want nil", *got.VerifySource)
	}
}

func TestSelectionToDomainPreservesExplicitVerifyCommand(t *testing.T) {
	verify := "fd --help"

	got := selectionToDomain(&selectionJSON{
		Asset:  "fd-v10.4.2-x86_64-unknown-linux-gnu.tar.gz",
		Verify: &verify,
	})

	if got.Verify == nil {
		t.Fatal("Verify is nil")
	}
	if *got.Verify != "fd --help" {
		t.Fatalf("Verify = %q", *got.Verify)
	}
}

func TestSelectionToDomainPreservesDocumentedInstallCommand(t *testing.T) {
	install := "cargo install fd-find"
	source := "readme"

	got := selectionToDomain(&selectionJSON{
		Asset:         "tool.tar.gz",
		Install:       &install,
		InstallSource: &source,
	})

	if got.Install == nil {
		t.Fatal("Install is nil")
	}
	if *got.Install != "cargo install fd-find" {
		t.Fatalf("Install = %q", *got.Install)
	}
	if got.InstallSource == nil || *got.InstallSource != domain.InstallSourceReadme {
		t.Fatalf("InstallSource = %v, want readme", got.InstallSource)
	}
}

func TestSelectionToDomainDropsBareInstallCommand(t *testing.T) {
	install := "codex"
	source := "readme"

	got := selectionToDomain(&selectionJSON{
		Asset:         "tool.tar.gz",
		Install:       &install,
		InstallSource: &source,
	})

	if got.Install != nil {
		t.Fatalf("Install = %q, want nil", *got.Install)
	}
	if got.InstallSource != nil {
		t.Fatalf("InstallSource = %q, want nil", *got.InstallSource)
	}
}

func TestSelectionToDomainInfersDeterministicAssetKind(t *testing.T) {
	got := selectionToDomain(&selectionJSON{Asset: "tool.AppImage"})

	if got.Kind == nil {
		t.Fatal("Kind is nil")
	}
	if *got.Kind != "appimage" {
		t.Fatalf("Kind = %q, want appimage", *got.Kind)
	}
}

func TestValidateSelectionsRejectsVerifySourceWithoutVerify(t *testing.T) {
	source := domain.VerifySourceReadme
	selections := mapForTest(&domain.PlatformSelection{
		Asset:        "tool.tar.gz",
		VerifySource: &source,
	})

	err := validateSelections(selections, []domain.ReleaseAsset{{Name: "tool.tar.gz"}})
	if err == nil {
		t.Fatal("validateSelections accepted verify_source without verify")
	}
}

func TestValidateSelectionsRejectsInstallSourceWithoutInstall(t *testing.T) {
	source := domain.InstallSourceReadme
	selections := mapForTest(&domain.PlatformSelection{
		Asset:         "tool.tar.gz",
		InstallSource: &source,
	})

	err := validateSelections(selections, []domain.ReleaseAsset{{Name: "tool.tar.gz"}})
	if err == nil {
		t.Fatal("validateSelections accepted install_source without install")
	}
}

func mapForTest(selection *domain.PlatformSelection) map[domain.Platform]*domain.PlatformSelection {
	result := map[domain.Platform]*domain.PlatformSelection{}
	for _, platform := range domain.Platforms {
		result[platform] = selection
	}
	return result
}
