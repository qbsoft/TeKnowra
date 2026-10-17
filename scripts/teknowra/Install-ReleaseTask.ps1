# Install only on the Windows deployment server, after Stage/Apply verification.
[CmdletBinding()]
param([string]$Root = 'D:\deploy\watchtower')
$ErrorActionPreference = 'Stop'
$name = 'TeKnowra-VerifiedRelease'
$scriptPath = Join-Path $Root 'Deploy-Release.ps1'
if (!(Test-Path $scriptPath) -or !(Test-Path (Join-Path $Root 'release-state.json'))) {
    throw 'Deploy script and a verified release-state.json are required.'
}
$existing = Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue
if ($existing) { Export-ScheduledTask -TaskName $name | Set-Content (Join-Path $Root 'previous-task.xml') -Encoding UTF8 }
$action = New-ScheduledTaskAction -Execute 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe' -Argument ('-NoProfile -ExecutionPolicy Bypass -File "' + $scriptPath + '" -Mode Apply -Root "' + $Root + '"') -WorkingDirectory $Root
$trigger = New-ScheduledTaskTrigger -Daily -At '23:00'
$settings = New-ScheduledTaskSettingsSet -MultipleInstances IgnoreNew -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 15) -ExecutionTimeLimit (New-TimeSpan -Hours 3) -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
$principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
Register-ScheduledTask -TaskName $name -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Description 'Resolve immutable releases, stage all images, update and verify all seven containers. Failure is nonzero; logs in D:\deploy\watchtower\releases.' -Force | Select-Object TaskName,State
