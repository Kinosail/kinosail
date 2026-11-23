param(
  [Parameter(Mandatory = $true)][string]$Msi,
  [Parameter(Mandatory = $true)][string]$Evidence
)
$ErrorActionPreference = 'Stop'
if ($env:CI -ne 'true' -or -not $IsWindows) { throw 'run on a disposable Windows CI host' }
$Msi = (Resolve-Path $Msi).Path
New-Item -ItemType Directory -Force -Path $Evidence | Out-Null
$Evidence = (Resolve-Path $Evidence).Path
$root = Join-Path $env:LOCALAPPDATA 'Kinosail'
$programs = Join-Path $env:LOCALAPPDATA 'Programs\Kinosail'
$runKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
$shortcut = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs\Kinosail.lnk'

trap {
  "$($_.InvocationInfo.PositionMessage)`n$($_.Exception.Message)" | Set-Content (Join-Path $Evidence 'failure.txt')
  Copy-Item -Path (Join-Path $root 'logs\*') -Destination $Evidence -ErrorAction SilentlyContinue
  Get-Process | Where-Object { $_.Name -like '*kinosail*' } | Format-Table Id, Name, StartTime -AutoSize | Out-String | Set-Content (Join-Path $Evidence 'processes.txt')
  Get-NetTCPConnection -LocalPort 38127 -ErrorAction SilentlyContinue | Out-String | Set-Content (Join-Path $Evidence 'port.txt')
  Get-Process | Where-Object { $_.Name -like '*kinosail*' } | Format-Table Id, Name, CPU, Threads, Handles -AutoSize | Out-String | Add-Content (Join-Path $Evidence 'processes.txt')
  "$script:healthError" | Set-Content (Join-Path $Evidence 'health-error.txt')
  curl.exe --insecure --verbose --max-time 5 https://127.0.0.1:38127/healthz *>&1 | Out-String | Set-Content (Join-Path $Evidence 'health-probe.txt')
  $server = Join-Path $programs 'server\kinosail-server.exe'
  if (Test-Path $server) {
    $env:KINOSAIL_DATA_DIR = Join-Path $root 'data'
    & $server healthcheck *>&1 | Out-String | Set-Content (Join-Path $Evidence 'healthcheck.txt')
    "exit $LASTEXITCODE" | Add-Content (Join-Path $Evidence 'healthcheck.txt')
  }
  break
}
function Step([string]$Text) {
  $line = "$((Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')) $Text"
  Write-Output $line
  Add-Content -Path (Join-Path $Evidence 'steps.log') -Value $line
}
function Test-Healthy {
  try {
    (Invoke-WebRequest -Uri 'https://127.0.0.1:38127/healthz' -SkipCertificateCheck -UseBasicParsing -TimeoutSec 5).StatusCode -eq 200
  } catch { $script:healthError = $_.Exception.ToString(); $false }
}
function Wait-Until([scriptblock]$Condition, [string]$Failure) {
  for ($i = 0; $i -lt 120; $i++) {
    if (& $Condition) { return }
    Start-Sleep -Milliseconds 500
  }
  throw $Failure
}
function Server { Get-Process -Name 'kinosail-server' -ErrorAction SilentlyContinue }
function Launcher { Get-Process -Name 'Kinosail' -ErrorAction SilentlyContinue }
function Invoke-Msi([string[]]$Arguments, [string]$Log) {
  $process = Start-Process -FilePath 'msiexec.exe' -ArgumentList ($Arguments + @('/qn', '/l*v', (Join-Path $Evidence $Log))) -PassThru
  $null = $process.Handle
  if (-not $process.WaitForExit(300000)) { throw "msiexec $($Arguments[0]) did not finish within 5 minutes" }
  if ($process.ExitCode -ne 0) { throw "msiexec $($Arguments[0]) failed with exit code $($process.ExitCode)" }
}

if (Test-Healthy) { throw 'another server already uses port 38127' }
New-Item -ItemType Directory -Force -Path $root | Out-Null
Set-Content -Path (Join-Path $root 'setup-opened') -Value '1'

Step 'install without an administrator prompt'
Invoke-Msi @('/i', $Msi) 'install.log'
foreach ($file in 'Kinosail.exe', 'Kinosail.ico', 'server\kinosail-server.exe', 'server\ffmpeg.exe', 'server\ffprobe.exe', 'server\fpcalc.exe') {
  if (-not (Test-Path (Join-Path $programs $file))) { throw "installed file is missing: $file" }
}
if (-not (Test-Path $shortcut)) { throw 'the Start menu shortcut is missing' }
if ((Get-ItemProperty -Path $runKey).Kinosail -ne "`"$programs\Kinosail.exe`"") { throw 'Kinosail does not start at sign-in' }

Step 'the installer starts Kinosail'
Wait-Until { Test-Healthy } 'Kinosail did not become healthy after install'
if (-not (Launcher)) { throw 'the tray app is not running' }
foreach ($folder in 'data', 'backups', 'cache', 'logs') {
  if (-not (Test-Path (Join-Path $root $folder))) { throw "data folder is missing: $folder" }
}

Step 'a second start does not run a second copy'
$second = Start-Process -FilePath (Join-Path $programs 'Kinosail.exe') -PassThru
if (-not $second.WaitForExit(15000)) { throw 'the second start did not exit' }
if (@(Launcher).Count -ne 1) { throw 'a second tray app kept running' }

Step 'restart the server after it crashes'
$first = (Server).Id
if (-not $first) { throw 'the server is not running' }
Stop-Process -Id $first -Force
Wait-Until { $current = Server; $current -and $current.Id -ne $first } 'the tray app did not restart the server'
Wait-Until { Test-Healthy } 'the restarted server did not become healthy'

Step 'uninstall stops Kinosail and keeps the library data'
Invoke-Msi @('/x', $Msi) 'uninstall.log'
Wait-Until { -not (Launcher) -and -not (Server) } 'Kinosail kept running after uninstall'
if (Test-Healthy) { throw 'port 38127 still answers after uninstall' }
if (Test-Path $programs) { throw 'program files remain after uninstall' }
if (Test-Path $shortcut) { throw 'the Start menu shortcut remains after uninstall' }
if ($null -ne (Get-ItemProperty -Path $runKey -Name Kinosail -ErrorAction SilentlyContinue)) { throw 'the sign-in entry remains after uninstall' }
if (-not (Test-Path (Join-Path $root 'data'))) { throw 'uninstall removed the library data' }
$log = Join-Path $root 'logs\kinosail.log'
Copy-Item $log (Join-Path $Evidence 'kinosail.log')
if (-not (Select-String -Path $log -SimpleMatch 'launcher closed; stopping' -Quiet)) { throw 'the server did not stop through the launcher pipe' }

Step 'passed'
$hashes = Get-FileHash -Algorithm SHA256 -Path $Msi, (Join-Path $Evidence 'steps.log'), (Join-Path $Evidence 'kinosail.log')
$hashes | ForEach-Object { "$($_.Hash.ToLower())  $(Split-Path $_.Path -Leaf)" } | Set-Content (Join-Path $Evidence 'evidence.sha256')
@{ revision = (git rev-parse HEAD); system = 'windows'; windows = [Environment]::OSVersion.VersionString; commands = @('scripts/stage-windows.sh', 'scripts/package-windows.sh', 'scripts/test-windows-installer.ps1') } |
  ConvertTo-Json -Compress | Set-Content (Join-Path $Evidence 'run.json')
