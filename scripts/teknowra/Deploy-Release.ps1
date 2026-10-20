# Windows PowerShell 5.1. Run on the production host; default is read-only.
[CmdletBinding()]
param(
    [ValidateSet('Plan','Stage','Apply')][string]$Mode = 'Plan',
    [string]$Root = 'D:\deploy\watchtower'
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$env:DOCKER_HOST = 'npipe:////./pipe/docker_engine'
$dockerCommand = Get-Command docker.exe -ErrorAction SilentlyContinue
$docker = if ($dockerCommand) { $dockerCommand.Source } else { 'D:\Program Files\Docker\resources\bin\docker.exe' }
if (!(Test-Path -LiteralPath $docker)) { throw 'Docker CLI not found.' }
$configDir = Join-Path $Root 'dockercfg'
$statePath = Join-Path $Root 'release-state.json'
$utf8 = [Text.UTF8Encoding]::new($false)
$lock = $null
$failed = $false

function Json([string]$Text, [string]$Label) {
    try { return ($Text | ConvertFrom-Json) }
    catch { throw "Cannot parse $Label JSON; raw values suppressed." }
}
function Docker([string[]]$Arguments) {
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try { $lines = & $script:docker @Arguments 2>&1; $code = $LASTEXITCODE }
    finally { $ErrorActionPreference = $previous }
    if ($code -ne 0) { throw ('docker ' + $Arguments[0] + ' failed: ' + ($lines -join "`n")) }
    return ($lines -join "`n")
}
function Api([string]$Path, [bool]$Private = $false) {
    $headers = @{'User-Agent'='TeKnowra-Release'; Accept='application/vnd.github+json'}
    # Both public workflow and private package reads must use the existing
    # credential; anonymous requests share a proxy IP's easily exhausted quota.
    $headers.Authorization = 'Bearer ' + $script:packageToken
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        try {
            $result = Invoke-RestMethod ('https://api.github.com/' + $Path) -Headers $headers -TimeoutSec 30 -MaximumRedirection 0
            return $result
        }
        catch { if ($attempt -eq 3) { throw }; Start-Sleep -Seconds 3 }
    }
}
function Package([string]$Name) {
    $versions = @(Api ('users/qbsoft/packages/container/' + $Name + '/versions?per_page=100') $true)
    $latest = @($versions | Where-Object { @($_.metadata.container.tags) -contains 'latest' })
    if ($latest.Count -ne 1) { throw "Cannot resolve unique latest package: $Name" }
    $sha = @($latest[0].metadata.container.tags | Where-Object { $_ -match '^[0-9a-f]{40}$' })
    if ($sha.Count -ne 1 -or $latest[0].name -notmatch '^sha256:[0-9a-f]{64}$') { throw "Invalid release metadata: $Name" }
    return @{Sha=$sha[0]; Digest=$latest[0].name; Created=$latest[0].created_at}
}
function Image([string]$Reference) {
    try { return @(Json (Docker @('image','inspect',$Reference)) 'image')[0] }
    catch { return $null }
}
function Pull([string]$Reference) {
    # Bound each CLI invocation; retry immutable references, never mutable latest.
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        Write-Host "Pull $Reference (attempt $attempt/3)"
        $outFile = Join-Path $script:runDir ('pull-' + [guid]::NewGuid().ToString('N') + '.out.log')
        $errFile = $outFile + '.err.log'
        $proc = Start-Process -FilePath $script:docker -ArgumentList @('--config', $script:configDir, 'pull', $Reference) -WindowStyle Hidden -PassThru -RedirectStandardOutput $outFile -RedirectStandardError $errFile
        $done = $proc.WaitForExit(600000)
        if (!$done) { Stop-Process -Id $proc.Id -ErrorAction SilentlyContinue }
        else { $proc.Refresh(); if ($proc.ExitCode -eq 0) { return } }
        if ($attempt -lt 3) { Start-Sleep -Seconds 10 }
    }
    throw "Image download failed after 3 attempts: $Reference. See $script:runDir. No containers have been changed."
}
function Probe($Entry) {
    $c = @(Json (Docker @('inspect',$Entry.Container)) 'container')[0]
    if (!$c.State.Running -or $c.Image -ne $Entry.Id) { return $false }
    if ($c.State.Health -and $c.State.Health.Status -ne 'healthy') { return $false }
    if ($Entry.Container -like 'tyer-mcp-*') {
        # Listener only: never invoke business tools (especially mail).
        $port = @{ 'tyer-mcp-crm'=8765; 'tyer-mcp-crm-review'=8766; 'tyer-mcp-mail'=8767 }[$Entry.Container]
        try { $null = Docker @('exec',$Entry.Container,'python','-c',"import socket; s=socket.create_connection(('127.0.0.1',$port),3); s.close()") }
        catch { return $false }
    }
    if ($Entry.Container -eq 'crm-backend') {
        try { $null = Docker @('exec',$Entry.Container,'sh','-c','wget -q --spider http://127.0.0.1:9999/admin/actuator/health') }
        catch { return $false }
    }
    return $true
}
function Refresh-Proxies {
    # nginx resolves static upstream names at config load. A recreated backend
    # can receive a different Docker IP even when its frontend image is unchanged.
    foreach ($frontendName in @('WeKnora-frontend','crm-frontend')) {
        $null = Docker @('exec',$frontendName,'nginx','-t')
        $null = Docker @('exec',$frontendName,'nginx','-s','reload')
    }
}
function Probe-ProxyRoutes {
    # Probe THROUGH nginx, not the backend health endpoint or SPA index.html.
    $body = Docker @('exec','WeKnora-frontend','wget','-qO-','http://127.0.0.1/api/v1/auth/oidc/config')
    $oidc = Json $body 'proxied OIDC config'
    if ($oidc.success -ne $true) { throw 'Proxied OIDC configuration failed.' }
    $health = Docker @('exec','crm-frontend','wget','-qO-','http://127.0.0.1:8888/admin/actuator/health')
    $crm = Json $health 'proxied CRM health'
    if ($crm.status -ne 'UP') { throw 'Proxied CRM health is not UP.' }
}
try {
    New-Item -ItemType Directory -Force -Path $Root | Out-Null
    $lock = [IO.File]::Open((Join-Path $Root 'release.lock'), 'OpenOrCreate', 'ReadWrite', 'None')
    $runDir = Join-Path $Root ('releases/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    New-Item -ItemType Directory -Force -Path $runDir | Out-Null
    Start-Transcript -Path (Join-Path $runDir 'deploy.log') | Out-Null
    $cfg = Json (Get-Content (Join-Path $configDir 'config.json') -Raw -Encoding UTF8) 'registry config'
    $auth = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($cfg.auths.'ghcr.io'.auth))
    $packageToken = $auth.Substring($auth.IndexOf(':') + 1)
    $runs = Api 'repos/qbsoft/TeKnowra/actions/workflows/teknowra-deploy.yml/runs?branch=main&status=success&per_page=1'
    $run = @($runs.workflow_runs)[0]
    if (!$run -or $run.head_sha -notmatch '^[0-9a-f]{40}$' -or $run.head_branch -ne 'main') { throw 'No successful TeKnowra main release.' }
    $mcp = Package 'tyer-mcp'
    $backend = Package 'crm-backend'
    $frontend = Package 'crm-frontend'
    if ($backend.Sha -ne $frontend.Sha) { throw 'CRM image publication is incomplete: backend/frontend commits differ. Retry later.' }
    $releases = @{
        TeKnowra = @{Sha=$run.head_sha; Created=$run.created_at}
        MCP = $mcp
        CRM = @{Sha=$backend.Sha; Created=$backend.Created}
    }
    if (Test-Path $statePath) {
        $old = Json (Get-Content $statePath -Raw -Encoding UTF8) 'release state'
        foreach ($key in @('TeKnowra','MCP','CRM')) {
            if ([datetime]$releases[$key].Created -lt [datetime]$old.Releases.$key.Created) { throw "Refusing older release for $key" }
        }
    }
    $entries = @(
        @{Container='WeKnora-app'; Repo='liugl2025/teknowra-app'; Sha=$run.head_sha},
        @{Container='WeKnora-frontend'; Repo='liugl2025/teknowra-ui'; Sha=$run.head_sha},
        @{Container='tyer-mcp-crm'; Repo='ghcr.io/qbsoft/tyer-mcp'; Sha=$mcp.Sha; Digest=$mcp.Digest},
        @{Container='tyer-mcp-crm-review'; Repo='ghcr.io/qbsoft/tyer-mcp'; Sha=$mcp.Sha; Digest=$mcp.Digest},
        @{Container='tyer-mcp-mail'; Repo='ghcr.io/qbsoft/tyer-mcp'; Sha=$mcp.Sha; Digest=$mcp.Digest},
        @{Container='crm-backend'; Repo='ghcr.io/qbsoft/crm-backend'; Sha=$backend.Sha; Digest=$backend.Digest},
        @{Container='crm-frontend'; Repo='ghcr.io/qbsoft/crm-frontend'; Sha=$frontend.Sha; Digest=$frontend.Digest}
    )
    foreach ($e in $entries) { Write-Host ($e.Container + ' -> ' + $e.Sha) }
    [IO.File]::WriteAllText((Join-Path $runDir 'plan.json'), (@{Releases=$releases; Entries=$entries} | ConvertTo-Json -Depth 8), $utf8)
    if ($Mode -eq 'Plan') { return }
    # Stage every image before moving any latest tag or restarting any service.
    foreach ($e in $entries) {
        $ref = $e.Repo + ':' + $e.Sha
        $source = if ($e.Digest) { $e.Repo + '@' + $e.Digest } else { $ref }
        $img = Image $source
        if (!$img) { Pull $source; $img = Image $source }
        if (!$img) { throw "Downloaded image missing: $source" }
        $e.Id = $img.Id
        $null = Docker @('tag',$e.Id,$ref)
    }
    if ($Mode -eq 'Stage') { Write-Host 'All images staged; no containers changed.'; return }
    $watchtower = @(Json (Docker @('inspect','watchtower')) 'watchtower')[0]
    if ($watchtower.State.Running) { throw 'Legacy watchtower is running. Disable its scheduler before applying releases.' }
    $before = Docker (@('inspect') + @($entries | ForEach-Object { $_.Container }))
    # Contains environment values; retain on this server only, never print or commit.
    [IO.File]::WriteAllText((Join-Path $runDir 'containers-before.json'), $before, $utf8)
    $containers = @(Json $before 'rollback snapshot')
    $changed = @()
    foreach ($e in $entries) {
        $c = @($containers | Where-Object { $_.Name -eq ('/' + $e.Container) })[0]
        if (!$c -or $c.Config.Image -ne ($e.Repo + ':latest')) { throw ('Unexpected image reference for ' + $e.Container) }
        if ($c.Image -ne $e.Id) { $changed += $e.Container }
        $null = Docker @('tag',$c.Image,('teknowra-rollback/' + $e.Container.ToLower() + ':' + (Split-Path $runDir -Leaf)))
    }
    foreach ($e in $entries) { $null = Docker @('tag',$e.Id,($e.Repo + ':latest')) }
    if ($changed.Count) {
        Write-Host ('Updating: ' + ($changed -join ', '))
        $output = Docker (@('run','--rm','--name','teknowra-release-updater','-v','/var/run/docker.sock:/var/run/docker.sock','-v',($configDir.Replace('\','/') + '/config.json:/config.json:ro'),$watchtower.Image,'--run-once','--no-pull') + $changed)
        Write-Host $output
        if ($output -match 'Failed=[1-9]') { throw 'Watchtower reported failed updates.' }
    }
    $deadline = (Get-Date).AddMinutes(5)
    do {
        $pending = @($entries | Where-Object { !(Probe $_) })
        if (!$pending.Count) { break }
        Start-Sleep -Seconds 10
    } while ((Get-Date) -lt $deadline)
    if ($pending.Count) { throw ('Post-deploy verification failed: ' + (($pending | ForEach-Object { $_.Container }) -join ', ')) }
    Refresh-Proxies
    # A graceful reload briefly leaves old workers serving existing connections.
    $proxyOK = $false
    for ($attempt = 1; $attempt -le 6; $attempt++) {
        try { Probe-ProxyRoutes; $proxyOK = $true; break }
        catch { if ($attempt -eq 6) { throw }; Start-Sleep -Seconds 2 }
    }
    if (!$proxyOK) { throw 'Proxy route verification failed.' }
    $state = @{VerifiedAt=(Get-Date).ToUniversalTime().ToString('o'); Releases=$releases; Entries=$entries; Backup=$runDir}
    [IO.File]::WriteAllText(($statePath + '.tmp'), ($state | ConvertTo-Json -Depth 8), $utf8)
    Move-Item -LiteralPath ($statePath + '.tmp') -Destination $statePath -Force
    Write-Host 'VERIFIED: all 7 containers match the resolved immutable releases.'
} catch {
    $failed = $true
    Write-Host ('FAILED: ' + $_.Exception.Message)
} finally {
    Remove-Variable packageToken,auth,cfg -ErrorAction SilentlyContinue
    try { Stop-Transcript | Out-Null } catch {}
    if ($lock) { $lock.Dispose() }
}
if ($failed) { exit 1 }
