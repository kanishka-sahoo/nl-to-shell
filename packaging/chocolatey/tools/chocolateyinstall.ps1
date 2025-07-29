$ErrorActionPreference = 'Stop'

$packageName = 'nl-to-shell'
$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$url64 = 'https://github.com/nl-to-shell/nl-to-shell/releases/download/0.1.0-dev/nl-to-shell-windows-amd64.exe'

$packageArgs = @{
  packageName   = $packageName
  unzipLocation = $toolsDir
  fileType      = 'exe'
  url64bit      = $url64
  softwareName  = 'nl-to-shell*'
  checksum64    = 'PLACEHOLDER_CHECKSUM'
  checksumType64= 'sha256'
  silentArgs    = '/S'
  validExitCodes= @(0)
}

Install-ChocolateyPackage @packageArgs
