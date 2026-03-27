# monitor
**Import:** `forge.lthn.ai/core/go-infra/cmd/monitor`
**Files:** 2

## Types

### `Finding`
Normalized security finding emitted by the `monitor` command regardless of source system.
- `Source string`: Source system or scanner name such as `semgrep`, `trivy`, or `dependabot`.
- `Severity string`: Normalized severity level.
- `Rule string`: Rule identifier, advisory identifier, or CVE.
- `File string`: Affected file path when the source provides one.
- `Line int`: Affected line number, or `0` when no location exists.
- `Message string`: Human-readable summary of the finding.
- `URL string`: Link to the upstream alert.
- `State string`: Alert state such as `open`, `dismissed`, `fixed`, or `resolved`.
- `RepoName string`: Short repository name used in output.
- `CreatedAt string`: Creation timestamp returned by GitHub.
- `Labels []string`: Suggested labels to attach downstream.

### `CodeScanningAlert`
Subset of the GitHub code scanning alert schema used by the command.
- `Number int`: Numeric GitHub alert ID.
- `State string`: Alert state.
- `Rule struct{ ID string; Severity string; Description string }`: Rule metadata returned by GitHub.
- `Tool struct{ Name string }`: Scanner or tool that emitted the alert.
- `MostRecentInstance struct{ Location struct{ Path string; StartLine int }; Message struct{ Text string } }`: Most recent code location and message payload attached to the alert.
- `HTMLURL string`: Browser URL for the alert.
- `CreatedAt string`: Creation timestamp.

### `DependabotAlert`
Subset of the GitHub Dependabot alert schema used by the command.
- `Number int`: Numeric GitHub alert ID.
- `State string`: Alert state.
- `SecurityVulnerability struct{ Severity string; Package struct{ Name string; Ecosystem string } }`: Vulnerability severity and affected package metadata.
- `SecurityAdvisory struct{ CVEID string; Summary string; Description string }`: Advisory identifiers and descriptive text.
- `Dependency struct{ ManifestPath string }`: Manifest file that introduced the vulnerable dependency.
- `HTMLURL string`: Browser URL for the alert.
- `CreatedAt string`: Creation timestamp.

### `SecretScanningAlert`
Subset of the GitHub secret scanning alert schema used by the command.
- `Number int`: Numeric GitHub alert ID.
- `State string`: Alert state.
- `SecretType string`: Secret or token classification.
- `Secret string`: Redacted secret preview from the API.
- `HTMLURL string`: Browser URL for the alert.
- `LocationType string`: Where GitHub found the secret.
- `CreatedAt string`: Creation timestamp.

## Functions

### `func AddMonitorCommands(root *cli.Command)`
Registers the top-level `monitor` command on the shared CLI root, along with its `repo`, `severity`, `json`, and `all` flags.
