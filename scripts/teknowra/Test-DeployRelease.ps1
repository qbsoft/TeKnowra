# Offline negative cases; no Docker, credentials, or network access.
$ErrorActionPreference = 'Stop'
$tokens = $null
$errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'Deploy-Release.ps1'), [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
foreach ($name in @('Json','Package','Probe','Refresh-Proxies','Probe-ProxyRoutes')) {
    $fn = $ast.Find({param($n) $n -is [Management.Automation.Language.FunctionDefinitionAst] -and $n.Name -eq $name}, $true)
    . ([scriptblock]::Create($fn.Extent.Text))
}
function Api { return $script:versions }
function Docker {
    param([string[]]$Arguments)
    if ($Arguments[0] -eq 'exec') { if ($script:listenerFails) { throw 'not listening' }; return '' }
    return ($script:container | ConvertTo-Json -Depth 6)
}
function Expect-Failure([scriptblock]$Action,[string]$Label) {
    $thrown=$false
    try { & $Action | Out-Null } catch { $thrown=$true }
    if (!$thrown) { throw ('Expected refusal: '+$Label) }
    Write-Host ('PASS: '+$Label)
}
$sha='a'*40
$digest='sha256:'+('b'*64)
$script:versions=@(
    @{name=$digest; created_at='2026-09-22T08:00:00Z'; metadata=@{container=@{tags=@('latest',$sha)}}},
    @{name=('sha256:'+('c'*64)); metadata=@{container=@{tags=@('d'*40)}}}
)
$p=Package 'tyer-mcp'
if ($p.Sha -ne $sha -or $p.Digest -ne $digest) { throw 'Wrong immutable release selection' }
Write-Host 'PASS: select one tagged manifest from a multi-version API response'
$script:versions=@(@{name=$digest; metadata=@{container=@{tags=@('latest')}}})
Expect-Failure {Package 'tyer-mcp'} 'reject latest without a commit tag'
$script:versions=@(@{name=$digest; metadata=@{container=@{tags=@('latest',$sha,('c'*40))}}})
Expect-Failure {Package 'tyer-mcp'} 'reject ambiguous commit tags'
$script:versions=@()
Expect-Failure {Package 'tyer-mcp'} 'reject missing release'
$entry=@{Container='tyer-mcp-mail'; Id='sha256:expected'}
$script:container=@{Image='sha256:old'; State=@{Running=$true}}
if (Probe $entry) { throw 'Old image falsely verified' }
Write-Host 'PASS: running container with old image is not success'
$script:container.Image=$entry.Id
$script:container.State.Running=$false
if (Probe $entry) { throw 'Stopped container falsely verified' }
Write-Host 'PASS: stopped target image is not success'
$script:container.State.Running=$true
$script:listenerFails=$true
if (Probe $entry) { throw 'Closed MCP listener falsely verified' }
Write-Host 'PASS: closed MCP listener is not success'
$script:listenerFails=$false
if (!(Probe $entry)) { throw 'Healthy target image refused' }
Write-Host 'PASS: exact image and ready listener accepted'
$script:proxyBroken=$true
$script:reloads=@()
function Docker {
    param([string[]]$Arguments)
    if ($Arguments -contains 'nginx') {
        $script:reloads += ($Arguments -join ' ')
        return ''
    }
    if ($Arguments[1] -eq 'WeKnora-frontend') {
        if ($script:proxyBroken) { return '<html>502 Bad Gateway</html>' }
        return '{"success":true,"enabled":true}'
    }
    if ($Arguments[1] -eq 'crm-frontend') { return '{"status":"UP"}' }
    throw 'Unexpected probe'
}
Expect-Failure {Probe-ProxyRoutes} 'reject healthy containers with broken nginx backend routing'
Refresh-Proxies
if ($script:reloads.Count -ne 4 -or $script:reloads[0] -notmatch 'nginx -t$' -or $script:reloads[1] -notmatch 'nginx -s reload$') { throw 'Missing validated proxy refresh' }
Write-Host 'PASS: validate and refresh both frontend upstream resolutions'
$script:proxyBroken=$false
Probe-ProxyRoutes
Write-Host 'PASS: actual backend JSON through both proxy routes is required'
