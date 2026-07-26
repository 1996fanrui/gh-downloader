package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/1996fanrui/gh-downloader/internal/domain"
)

type OpenAIAnalyzer struct {
	apiKey string
	base   string
	model  string
	client *http.Client
}

func NewOpenAIAnalyzer(apiKey string, base string, model string, client *http.Client) *OpenAIAnalyzer {
	return &OpenAIAnalyzer{apiKey: apiKey, base: strings.TrimRight(base, "/"), model: model, client: client}
}

func (a *OpenAIAnalyzer) Describe(ctx context.Context, input DescriptionInput) (string, error) {
	content, err := a.completeJSON(ctx, []chatMessage{
		{Role: "system", Content: descriptionSystemPrompt()},
		{Role: "user", Content: descriptionUserPrompt(input)},
	}, descriptionResponseFormat())
	if err != nil {
		return "", err
	}

	var out descriptionResponse
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return "", fmt.Errorf("decode description JSON: %w", err)
	}
	description := strings.TrimSpace(out.Description)
	if err := validateDescription(description); err != nil {
		return "", err
	}
	return description, nil
}

func (a *OpenAIAnalyzer) InstallMethods(ctx context.Context, input InstallMethodsInput) ([]domain.InstallMethod, error) {
	content, err := a.completeJSON(ctx, []chatMessage{
		{Role: "system", Content: installMethodsSystemPrompt()},
		{Role: "user", Content: installMethodsUserPrompt(input)},
	}, installMethodsResponseFormat())
	if err != nil {
		return nil, err
	}

	var out installMethodsResponse
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("decode install methods JSON: %w", err)
	}
	return normalizeInstallMethods(out.Methods), nil
}

func (a *OpenAIAnalyzer) Analyze(ctx context.Context, input Input) (map[domain.Platform]*domain.PlatformSelection, error) {
	content, err := a.completeJSON(ctx, []chatMessage{
		{Role: "system", Content: systemPrompt()},
		{Role: "user", Content: userPrompt(input)},
	}, responseFormat())
	if err != nil {
		return nil, err
	}

	var analysis analysisResponse
	if err := json.Unmarshal([]byte(content), &analysis); err != nil {
		return nil, fmt.Errorf("decode analysis JSON: %w", err)
	}
	selections := analysis.toDomain()
	if err := validateSelections(selections, input.Assets); err != nil {
		return nil, err
	}
	return selections, nil
}

func (a *OpenAIAnalyzer) completeJSON(ctx context.Context, messages []chatMessage, format json.RawMessage) (string, error) {
	if strings.TrimSpace(a.apiKey) == "" {
		return "", errors.New("OPENAI_API_KEY is required for release analysis")
	}
	if strings.TrimSpace(a.base) == "" {
		return "", errors.New("OPENAI_BASE_URL is required for release analysis")
	}
	if strings.TrimSpace(a.model) == "" {
		return "", errors.New("OPENAI_MODEL is required for release analysis")
	}

	payload := chatRequest{
		Model:          a.model,
		Messages:       messages,
		ResponseFormat: format,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal OpenAI request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create OpenAI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call OpenAI: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read OpenAI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("OpenAI returned %s: %s", resp.Status, string(respBody))
	}

	var out chatResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("decode OpenAI response: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", errors.New("OpenAI returned no choices")
	}
	return out.Choices[0].Message.Content, nil
}

func validateSelections(selections map[domain.Platform]*domain.PlatformSelection, assets []domain.ReleaseAsset) error {
	names := map[string]struct{}{}
	for _, asset := range assets {
		names[asset.Name] = struct{}{}
	}

	for _, platform := range domain.Platforms {
		selection, ok := selections[platform]
		if !ok {
			return fmt.Errorf("analysis missing platform %s", platform)
		}
		if selection == nil {
			continue
		}
		if _, ok := names[selection.Asset]; !ok {
			return fmt.Errorf("analysis selected unknown asset %q for %s", selection.Asset, platform)
		}
		if selection.Kind != nil && !IsValidKind(*selection.Kind) {
			return fmt.Errorf("analysis selected invalid kind %q for %s", *selection.Kind, platform)
		}
		if selection.InstallSource != nil && selection.Install == nil {
			return fmt.Errorf("analysis selected install_source without install for %s", platform)
		}
		if selection.InstallSource != nil && !IsValidInstallSource(*selection.InstallSource) {
			return fmt.Errorf("analysis selected invalid install_source %q for %s", *selection.InstallSource, platform)
		}
		if selection.VerifySource != nil && selection.Verify == nil {
			return fmt.Errorf("analysis selected verify_source without verify for %s", platform)
		}
		if selection.VerifySource != nil && !IsValidVerifySource(*selection.VerifySource) {
			return fmt.Errorf("analysis selected invalid verify_source %q for %s", *selection.VerifySource, platform)
		}
	}
	return nil
}

func systemPrompt() string {
	return "You map GitHub release assets to one best end-user download per platform. Return only JSON that matches the schema. Select only primary user-facing products. Exclude signatures, checksums, debug symbols, manifests, blockmaps, source archives, and internal components. If a platform has no primary asset, return null. The program handles deterministic file-format rules, so set kind only when README or release context is needed to disambiguate the asset, such as binary for a standalone CLI executable or macapp for a macOS .app archive. Only return install when the README explicitly documents a command block for installing the selected asset; set install_source to readme only when install is present. Do not return generic download, extract, chmod, open, PATH, or package-manager commands that are not explicitly documented by the README. Only return verify when the README explicitly documents a non-launching verification command for the installed tool; set verify_source to readme only when verify is present. Do not infer command names from repository names or filenames. Do not assume --version or --help is supported. Never return a bare executable name as install or verify."
}

func descriptionSystemPrompt() string {
	return "Write one concise Simplified Chinese description for a GitHub repository. Return only JSON that matches the schema. The description must be a plain user-facing Chinese sentence, no markdown, no emoji, no quotes, no trailing punctuation beyond a single Chinese full stop. Keep unavoidable product or company names as-is, but do not copy an English GitHub description verbatim. Prefer 12 to 36 Chinese characters when possible."
}

func installMethodsSystemPrompt() string {
	return "Extract project-level, version-independent install methods for end users from the README. Return only JSON that matches the schema. Return README-backed candidates; do not rank them. Exclude commands from Development, Contributing, build-from-source, package-maintainer, test, CI, or local repository setup sections. Exclude dependency/bootstrap commands such as npm install, npm ci, npm run build, uv pip install -e, pip install -e, or commands installing .[dev] extras. Include official remote installer scripts only when the README presents them as an end-user install path, and set manager to remote_script for curl|sh, curl|bash, wget|sh, wget|bash, irm|iex, iwr|iex, or equivalent commands that pipe downloaded remote script text into a shell. Exclude manual GitHub Release download, extract, chmod, PATH, --version, and --help commands. Include only short install commands that install the published product for a user. Set manager to one of: npm, homebrew, cargo, pipx, winget, scoop, choco, apt, dnf, yum, pacman, zypper, apk, mise, nix, guix, macports, remote_script, other. usage_command is an optional command the README explicitly says users can run after installation, including first use, launch, or verification. If README states a global usage command, repeat it for every install method unless a method-specific command is documented. Do not invent usage_command. Do not append --version or --help unless README uses that exact command. Use concise Simplified Chinese titles. Use platforms to mark where each command applies; use all six platforms for cross-platform commands such as npm. If README does not explicitly document any install command for end users, return an empty methods array."
}

func userPrompt(input Input) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Repository: %s\nRelease tag: %s\n\nAssets:\n", input.RepoFullName, input.Tag)
	for _, asset := range input.Assets {
		fmt.Fprintf(&b, "- %s\n", asset.Name)
	}
	fmt.Fprintf(&b, "\nREADME:\n%s\n", input.Readme)
	return b.String()
}

func descriptionUserPrompt(input DescriptionInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Repository: %s\n", input.RepoFullName)
	fmt.Fprintf(&b, "GitHub description: %s\n", strings.TrimSpace(input.GitHubDescription))
	fmt.Fprintf(&b, "\nREADME:\n%s\n", input.Readme)
	return b.String()
}

func installMethodsUserPrompt(input InstallMethodsInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Repository: %s\n", input.RepoFullName)
	fmt.Fprintf(&b, "\nREADME:\n%s\n", input.Readme)
	return b.String()
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat json.RawMessage `json:"response_format"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type analysisResponse struct {
	WinX64     *selectionJSON `json:"win_x64"`
	WinARM64   *selectionJSON `json:"win_arm64"`
	MacX64     *selectionJSON `json:"mac_x64"`
	MacARM64   *selectionJSON `json:"mac_arm64"`
	LinuxX64   *selectionJSON `json:"linux_x64"`
	LinuxARM64 *selectionJSON `json:"linux_arm64"`
}

type descriptionResponse struct {
	Description string `json:"description"`
}

type installMethodsResponse struct {
	Methods []installMethodJSON `json:"methods"`
}

type installMethodJSON struct {
	Title        string   `json:"title"`
	Manager      string   `json:"manager"`
	Command      string   `json:"command"`
	UsageCommand *string  `json:"usage_command"`
	Platforms    []string `json:"platforms"`
	Source       string   `json:"source"`
	Recommended  bool     `json:"recommended"`
}

type selectionJSON struct {
	Asset         string  `json:"asset"`
	Kind          *string `json:"kind"`
	Install       *string `json:"install"`
	InstallSource *string `json:"install_source"`
	Verify        *string `json:"verify"`
	VerifySource  *string `json:"verify_source"`
	Note          *string `json:"note"`
}

func validateDescription(description string) error {
	if description == "" {
		return errors.New("analysis returned empty repository description")
	}
	if !containsCJK(description) {
		return fmt.Errorf("analysis returned non-Chinese repository description %q", description)
	}
	if strings.Contains(description, "\n") || strings.Contains(description, "\r") {
		return fmt.Errorf("analysis returned multiline repository description %q", description)
	}
	return nil
}

func containsCJK(value string) bool {
	for _, r := range value {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}

func normalizeInstallMethods(values []installMethodJSON) []domain.InstallMethod {
	methods := make([]domain.InstallMethod, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		command := normalizeDocumentedCommand(&value.Command)
		if command == nil {
			continue
		}
		title := strings.TrimSpace(value.Title)
		if title == "" {
			title = "Official install"
		}
		source := domain.InstallSource(value.Source)
		if !IsValidInstallSource(source) {
			continue
		}
		manager := normalizeManager(value.Manager)
		if manager == "" || isDevelopmentInstallCommand(*command) {
			continue
		}
		if isRemoteScriptCommand(*command) {
			manager = "remote_script"
		}
		if !commandMatchesManager(*command, manager) {
			continue
		}
		key := title + "\x00" + *command
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		methods = append(methods, domain.InstallMethod{
			Title:        title,
			Manager:      manager,
			Command:      *command,
			UsageCommand: normalizeUsageCommand(value.UsageCommand),
			Platforms:    normalizePlatforms(value.Platforms),
			Source:       source,
			Recommended:  value.Recommended,
		})
	}
	return methods
}

func normalizeManager(value string) string {
	manager := strings.ToLower(strings.TrimSpace(value))
	switch manager {
	case "npm", "homebrew", "cargo", "pipx", "winget", "scoop", "choco", "apt", "dnf", "yum", "pacman", "zypper", "apk", "mise", "nix", "guix", "macports", "remote_script", "other":
		return manager
	default:
		return ""
	}
}

func commandMatchesManager(command string, manager string) bool {
	first := firstCommandName(command)
	switch manager {
	case "homebrew":
		return first == "brew"
	case "macports":
		return first == "port"
	case "remote_script":
		return isRemoteScriptCommand(command)
	case "other":
		return true
	default:
		return first == manager
	}
}

func firstCommandName(command string) string {
	fields := commandFields(command)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToLower(fields[0])
}

func commandFields(command string) []string {
	fields := strings.Fields(firstCommandLine(command))
	for len(fields) > 0 {
		value := fields[0]
		switch {
		case value == "sudo" || value == "doas" || value == "env":
			fields = fields[1:]
		case strings.Contains(value, "=") && !strings.HasPrefix(value, "-"):
			fields = fields[1:]
		default:
			return fields
		}
	}
	return nil
}

func firstCommandLine(command string) string {
	for _, line := range strings.Split(command, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed
	}
	return ""
}

func isRemoteScriptCommand(command string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(command), " "))
	if hasAnyRemoteDownloader(normalized) && hasPowerShellExpressionInvoker(normalized) {
		return true
	}
	for _, pattern := range []string{
		"curl ", "wget ",
	} {
		if !strings.Contains(normalized, pattern) {
			continue
		}
		if strings.Contains(normalized, "| sh") ||
			strings.Contains(normalized, "| bash") ||
			strings.Contains(normalized, "| zsh") ||
			strings.Contains(normalized, "| iex") ||
			strings.Contains(normalized, "invoke-expression") {
			return true
		}
	}
	return false
}

func hasAnyRemoteDownloader(command string) bool {
	for _, pattern := range []string{"curl ", "wget ", "irm ", "iwr ", "invoke-webrequest ", "invoke-restmethod "} {
		if strings.Contains(command, pattern) {
			return true
		}
	}
	return false
}

func hasPowerShellExpressionInvoker(command string) bool {
	return strings.Contains(command, "| iex") ||
		strings.Contains(command, "iex ") ||
		strings.Contains(command, "iex(") ||
		strings.Contains(command, "invoke-expression")
}

func isDevelopmentInstallCommand(command string) bool {
	fields := lowerCommandFields(command)
	if len(fields) == 0 {
		return false
	}
	return isNPMDependencyInstallCommand(fields) || isEditablePythonInstallCommand(fields)
}

func lowerCommandFields(command string) []string {
	fields := commandFields(command)
	for i, field := range fields {
		fields[i] = strings.ToLower(strings.Trim(field, `"'`))
	}
	return fields
}

func isNPMDependencyInstallCommand(fields []string) bool {
	if len(fields) < 2 || fields[0] != "npm" {
		return false
	}
	if fields[1] == "ci" {
		return true
	}
	if fields[1] != "install" && fields[1] != "i" {
		return false
	}

	hasPackage := false
	global := false
	for _, field := range fields[2:] {
		switch {
		case field == "-g" || field == "--global":
			global = true
		case field == "--":
			continue
		case strings.HasPrefix(field, "-"):
			continue
		default:
			hasPackage = true
		}
	}
	return !global || !hasPackage
}

func isEditablePythonInstallCommand(fields []string) bool {
	start := -1
	switch {
	case len(fields) >= 3 && fields[0] == "pip" && fields[1] == "install":
		start = 2
	case len(fields) >= 4 && fields[0] == "python" && fields[1] == "-m" && fields[2] == "pip" && fields[3] == "install":
		start = 4
	case len(fields) >= 4 && fields[0] == "uv" && fields[1] == "pip" && fields[2] == "install":
		start = 3
	default:
		return false
	}

	for _, field := range fields[start:] {
		if field == "-e" || field == "--editable" || field == "." || strings.HasPrefix(field, ".[") {
			return true
		}
		if strings.Contains(field, "dev") && strings.Contains(field, "[") {
			return true
		}
	}
	return false
}

func normalizeUsageCommand(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizePlatforms(values []string) []domain.Platform {
	allowed := map[domain.Platform]struct{}{}
	for _, platform := range domain.Platforms {
		allowed[platform] = struct{}{}
	}
	result := make([]domain.Platform, 0, len(values))
	seen := map[domain.Platform]struct{}{}
	for _, value := range values {
		platform := domain.Platform(strings.TrimSpace(value))
		if _, ok := allowed[platform]; !ok {
			continue
		}
		if _, ok := seen[platform]; ok {
			continue
		}
		seen[platform] = struct{}{}
		result = append(result, platform)
	}
	return result
}

func (a analysisResponse) toDomain() map[domain.Platform]*domain.PlatformSelection {
	return map[domain.Platform]*domain.PlatformSelection{
		domain.PlatformWinX64:     selectionToDomain(a.WinX64),
		domain.PlatformWinARM64:   selectionToDomain(a.WinARM64),
		domain.PlatformMacX64:     selectionToDomain(a.MacX64),
		domain.PlatformMacARM64:   selectionToDomain(a.MacARM64),
		domain.PlatformLinuxX64:   selectionToDomain(a.LinuxX64),
		domain.PlatformLinuxARM64: selectionToDomain(a.LinuxARM64),
	}
}

func selectionToDomain(value *selectionJSON) *domain.PlatformSelection {
	if value == nil {
		return nil
	}
	var kind *domain.AssetKind
	if inferred := InferKind(value.Asset); inferred != nil {
		kind = inferred
	} else if value.Kind != nil {
		k := domain.AssetKind(*value.Kind)
		kind = &k
	}
	install := normalizeDocumentedCommand(value.Install)
	var installSource *domain.InstallSource
	if install != nil && value.InstallSource != nil {
		s := domain.InstallSource(*value.InstallSource)
		installSource = &s
	}
	verify := normalizeVerifyCommand(value.Verify)
	var source *domain.VerifySource
	if verify != nil && value.VerifySource != nil {
		s := domain.VerifySource(*value.VerifySource)
		source = &s
	}
	return &domain.PlatformSelection{
		Asset:         value.Asset,
		Kind:          kind,
		Install:       install,
		InstallSource: installSource,
		Verify:        verify,
		VerifySource:  source,
		Note:          value.Note,
	}
}

func normalizeVerifyCommand(value *string) *string {
	return normalizeDocumentedCommand(value)
}

func normalizeDocumentedCommand(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	if isBareExecutable(trimmed) {
		return nil
	}
	return &trimmed
}

func isBareExecutable(value string) bool {
	if strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '.', '_', '-', '/':
			continue
		default:
			return false
		}
	}
	return true
}

func responseFormat() json.RawMessage {
	return json.RawMessage(`{
  "type": "json_schema",
  "json_schema": {
    "name": "release_asset_platform_map",
    "strict": true,
    "schema": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "win_x64": {"$ref": "#/$defs/selection_or_null"},
        "win_arm64": {"$ref": "#/$defs/selection_or_null"},
        "mac_x64": {"$ref": "#/$defs/selection_or_null"},
        "mac_arm64": {"$ref": "#/$defs/selection_or_null"},
        "linux_x64": {"$ref": "#/$defs/selection_or_null"},
        "linux_arm64": {"$ref": "#/$defs/selection_or_null"}
      },
      "required": ["win_x64", "win_arm64", "mac_x64", "mac_arm64", "linux_x64", "linux_arm64"],
      "$defs": {
        "selection_or_null": {
          "anyOf": [
            {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "asset": {"type": "string"},
                "kind": {"anyOf": [{"type": "string", "enum": ["installer", "binary", "appimage", "archive", "macapp"]}, {"type": "null"}]},
                "install": {"anyOf": [{"type": "string"}, {"type": "null"}]},
                "install_source": {"anyOf": [{"type": "string", "enum": ["readme"]}, {"type": "null"}]},
                "verify": {"anyOf": [{"type": "string"}, {"type": "null"}]},
                "verify_source": {"anyOf": [{"type": "string", "enum": ["readme"]}, {"type": "null"}]},
                "note": {"anyOf": [{"type": "string"}, {"type": "null"}]}
              },
              "required": ["asset", "kind", "install", "install_source", "verify", "verify_source", "note"]
            },
            {"type": "null"}
          ]
        }
      }
    }
  }
}`)
}

func descriptionResponseFormat() json.RawMessage {
	return json.RawMessage(`{
  "type": "json_schema",
  "json_schema": {
    "name": "repository_description",
    "strict": true,
    "schema": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "description": {"type": "string"}
      },
      "required": ["description"]
    }
  }
}`)
}

func installMethodsResponseFormat() json.RawMessage {
	return json.RawMessage(`{
  "type": "json_schema",
  "json_schema": {
    "name": "project_install_methods",
    "strict": true,
    "schema": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "methods": {
          "type": "array",
          "items": {
            "type": "object",
            "additionalProperties": false,
            "properties": {
              "title": {"type": "string"},
              "manager": {"type": "string", "enum": ["npm", "homebrew", "cargo", "pipx", "winget", "scoop", "choco", "apt", "dnf", "yum", "pacman", "zypper", "apk", "mise", "nix", "guix", "macports", "remote_script", "other"]},
              "command": {"type": "string"},
              "usage_command": {"anyOf": [{"type": "string"}, {"type": "null"}]},
              "platforms": {
                "type": "array",
                "items": {"type": "string", "enum": ["win/x64", "win/arm64", "mac/x64", "mac/arm64", "linux/x64", "linux/arm64"]}
              },
              "source": {"type": "string", "enum": ["readme"]},
              "recommended": {"type": "boolean"}
            },
            "required": ["title", "manager", "command", "usage_command", "platforms", "source", "recommended"]
          }
        }
      },
      "required": ["methods"]
    }
  }
}`)
}
