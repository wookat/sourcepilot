# Phase R1.2-Auto — Documentation consistency checks.
# Usage: .\scripts\check-doc-links.ps1 [-OutFile docs/DOCS_CONSISTENCY_CHECK.md]

param(
    [string]$OutFile = "docs/DOCS_CONSISTENCY_CHECK.md"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$issues = @()
$passed = 0
$failed = 0

function Add-Issue {
    param([string]$Check, [bool]$Ok, [string]$Detail)
    $script:issues += @{ check = $Check; passed = $Ok; detail = $Detail }
    if ($Ok) { $script:passed++ } else { $script:failed++ }
    $mark = if ($Ok) { "PASS" } else { "FAIL" }
    Write-Host ("  [{0}] {1} - {2}" -f $mark, $Check, $Detail)
}

Write-Host "Documentation consistency check (Phase R1.2-Auto)..."

$wrongRouteHits = @()
Get-ChildItem -Path $repoRoot -Recurse -Include *.md,*.tsx,*.ts -ErrorAction SilentlyContinue |
    Where-Object {
        $_.FullName -notmatch '\\node_modules\\|\\\.umi|\\dist\\|\\backend\\'
    } | ForEach-Object {
        $lines = Get-Content $_.FullName -ErrorAction SilentlyContinue
        for ($i = 0; $i -lt $lines.Count; $i++) {
            $line = $lines[$i]
            if ($line -match '`/task-center/failures`|"/task-center/failures"|''/task-center/failures''' -and $line -notmatch '/ops/task-center/failures|/api/v1/task-center') {
                $rel = $_.FullName.Substring($repoRoot.Length + 1)
                $wrongRouteHits += "${rel}:$($i+1)"
            }
        }
    }
Add-Issue "Admin route uses /ops/task-center/failures" ($wrongRouteHits.Count -eq 0) $(if ($wrongRouteHits.Count -eq 0) { "ok" } else { ($wrongRouteHits | Select-Object -First 5) -join ", " })

# Per .cursor/rules/15-external-docs-no-phase-status.mdc, README.md / README.en.md
# must NOT carry phase/stage status banners; phase progress lives in docs/PROGRESS.md.
foreach ($readmeName in @("README.md", "README.en.md")) {
    $readme = Get-Content (Join-Path $repoRoot $readmeName) -Raw
    Add-Issue "$readmeName has no phase status banner" ($readme -notmatch 'MVP Demo Ready|Production Capability Development|Post-F9 Enhancement|Tag deferred|Release Candidate|Release Status|Not Production Ready') "$readmeName phase status"
}

$prodReadyBad = @()
Get-ChildItem -Path (Join-Path $repoRoot "docs") -Filter *.md -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
    $lines = Get-Content $_.FullName -ErrorAction SilentlyContinue
    foreach ($line in $lines) {
        if ($line -match 'Production Ready' -and $line -notmatch '非 Production|not Production|Non Production|非 production') {
            $prodReadyBad += "$($_.Name): $line"
            break
        }
    }
}
Add-Issue "No unqualified Production Ready in docs/" ($prodReadyBad.Count -eq 0) $(if ($prodReadyBad.Count -eq 0) { "ok" } else { ($prodReadyBad | Select-Object -First 5) -join ", " })

$apiMd = Join-Path $repoRoot "docs/api.md"
$apiContent = if (Test-Path $apiMd) { Get-Content $apiMd -Raw } else { "" }
Add-Issue "docs/api.md documents task-center failures API" ($apiContent -match '/api/v1/task-center/failures') "api.md"
Add-Issue "docs/api.md documents operation-workbench API" ($apiContent -match '/api/v1/ai/operation-workbench/summary') "api.md"

$conclusion = if ($failed -eq 0) { "passed" } else { "failed" }
$generatedAt = (Get-Date).ToUniversalTime().ToString("o")
$md = @(
    "# Documentation Consistency Check (Phase R1.2-Auto)",
    "",
    "> Generated: $generatedAt",
    "",
    "## Result: $(if ($conclusion -eq 'passed') { 'PASS' } else { 'FAIL' })",
    "",
    "| Check | Result | Detail |",
    "| --- | --- | --- |"
)
foreach ($item in $issues) {
    $result = if ($item.passed) { "PASS" } else { "FAIL" }
    $md += "| $($item.check) | $result | $($item.detail) |"
}
$md += @(
    "",
    "## Release status policy",
    "",
    "- README.md / README.en.md carry no phase/stage status (rule 15-external-docs-no-phase-status).",
    "- Phase progress and acceptance status live in docs/PROGRESS.md and phase reports.",
    "",
    "## Route convention",
    "",
    "- Frontend: /ops/task-center/failures",
    "- API: /api/v1/task-center/failures",
    ""
)

$dir = Split-Path -Parent $OutFile
if ($dir -and -not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
($md -join "`n").TrimEnd() | Set-Content -Path $OutFile -Encoding UTF8
Write-Host ""
Write-Host "Wrote $OutFile ($passed passed, $failed failed)"
if ($failed -gt 0) { exit 1 }
exit 0
