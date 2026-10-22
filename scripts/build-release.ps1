#requires -Version 7.4
param(
    [Parameter(Mandatory)]
    [ValidatePattern('^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$')]
    [string]$Tag
)

$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true
$version = [version]($Tag.Substring(1) + '.0')
if ($version.Major -lt 1 -or $version.Major -gt 65535 -or $version.Minor -gt 65535 -or $version.Build -gt 65535) {
    throw 'Use vMAJOR.MINOR.PATCH with major 1–65535 and minor/patch 0–65535.'
}

$root = Split-Path $PSScriptRoot -Parent
$rc = Join-Path $env:ProgramFiles 'LLVM/bin/llvm-rc.exe'
$objcopy = Join-Path $env:ProgramFiles 'LLVM/bin/llvm-objcopy.exe'
$vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio/Installer/vswhere.exe'
if (!(Test-Path $rc)) { throw 'LLVM llvm-rc.exe was not found.' }
if (!(Test-Path $objcopy)) { throw 'LLVM llvm-objcopy.exe was not found.' }
if (!(Test-Path $vswhere)) { throw 'Visual Studio Installer vswhere.exe was not found.' }
$cvtres = & $vswhere -latest -products '*' -find 'VC\Tools\MSVC\*\bin\Hostx64\x64\cvtres.exe' |
    Select-Object -First 1
if (!$cvtres) { throw 'MSVC cvtres.exe was not found. Install the Visual Studio C++ build tools.' }

$dist = Join-Path $root 'dist'
if (Test-Path $dist) { throw 'Remove the existing dist directory before building another release.' }
$build = New-Item "$dist/.build" -ItemType Directory -Force
Copy-Item "$root/*.go", "$root/go.mod", "$root/assets/icon.ico" $build
$goLicense = Join-Path (go env GOROOT) 'LICENSE'

# Build in a temporary directory so checked-in resources stay untouched.
$resource = Get-Content "$root/assets/resources.rc" -Raw
$resource = $resource -replace '(?m)^(FILEVERSION|PRODUCTVERSION) .+$', ('${1} ' + $version.ToString().Replace('.', ','))
$resource = $resource -replace '(VALUE "(?:FileVersion|ProductVersion)", ")[^"]*', ('${1}' + $version + '\0')
Set-Content "$build/resources.rc" $resource -Encoding utf8
Push-Location $build
try {
    & $rc /no-preprocess /fo resources.res resources.rc
    foreach ($arch in 'amd64', 'arm64') {
        $machine = if ($arch -eq 'amd64') { 'x64' } else { 'arm64' }
        & $cvtres /nologo "/machine:$machine" "/out:resources_windows_$arch.syso" resources.res
        # Remove MSVC compiler metadata that Go's linker does not accept.
        & $objcopy --strip-debug --strip-symbol=@comp.id "resources_windows_$arch.syso"
        $env:GOOS = 'windows'
        $env:GOARCH = $arch
        $env:CGO_ENABLED = '0'
        go vet .
        go build -trimpath -ldflags='-H=windowsgui -s -w' -o "$dist/smol-auto-clicker-$Tag-windows-$machine.exe" .
    }
} finally {
    Pop-Location
}

$identityValues = @($env:STORE_IDENTITY_NAME, $env:STORE_PUBLISHER, $env:STORE_PUBLISHER_DISPLAY_NAME)
$configured = @($identityValues | Where-Object { ![string]::IsNullOrWhiteSpace($_) }).Count
if ($configured -ne 0 -and $configured -ne 3) {
    throw 'Set all three Store identity variables, or leave all three unset for portable-only releases.'
}
if ($configured -eq 3) {
    $sdk = Get-ChildItem "${env:ProgramFiles(x86)}/Windows Kits/10/bin" -Directory |
        Where-Object { $_.Name -match '^10\.0\.\d+\.0$' } |
        Sort-Object { [version]$_.Name } -Descending |
        Where-Object { Test-Path "$($_.FullName)/x64/makeappx.exe" } |
        Select-Object -First 1
    if (!$sdk) { throw 'Windows SDK MakeAppx.exe was not found.' }
    $makeappx = "$($sdk.FullName)/x64/makeappx.exe"
    $packages = New-Item "$build/packages" -ItemType Directory
    Add-Type -AssemblyName System.Drawing
    $image = [System.Drawing.Image]::FromFile("$root/assets/icon.png")
    try {
        foreach ($arch in 'x64', 'arm64') {
            $package = New-Item "$build/package-$arch" -ItemType Directory
            Copy-Item "$dist/smol-auto-clicker-$Tag-windows-$arch.exe" "$package/smol-auto-clicker.exe"
            Copy-Item "$root/LICENSE" $package
            Copy-Item $goLicense "$package/GO-LICENSE.txt"
            $assets = New-Item "$package/Assets" -ItemType Directory
            foreach ($logo in @{ StoreLogo = 50; Square44x44Logo = 44; Square150x150Logo = 150 }.GetEnumerator()) {
                $bitmap = [System.Drawing.Bitmap]::new($logo.Value, $logo.Value)
                $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
                try {
                    $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
                    $graphics.DrawImage($image, 0, 0, $logo.Value, $logo.Value)
                    $bitmap.Save("$assets/$($logo.Key).png", [System.Drawing.Imaging.ImageFormat]::Png)
                } finally {
                    $graphics.Dispose()
                    $bitmap.Dispose()
                }
            }
            [xml]$manifest = Get-Content "$root/packaging/AppxManifest.xml" -Raw
            $manifest.Package.Identity.Name = $env:STORE_IDENTITY_NAME
            $manifest.Package.Identity.Publisher = $env:STORE_PUBLISHER
            $manifest.Package.Identity.Version = $version.ToString()
            $manifest.Package.Identity.ProcessorArchitecture = $arch
            $manifest.Package.Properties.PublisherDisplayName = $env:STORE_PUBLISHER_DISPLAY_NAME
            $manifest.Save("$package/AppxManifest.xml")
            & $makeappx pack /d $package.FullName /p "$packages/$arch.msix" /o
        }
    } finally {
        $image.Dispose()
    }
    & $makeappx bundle /d $packages.FullName /p "$dist/smol-auto-clicker-$Tag-store.msixbundle" /bv "$version" /o
} else {
    Write-Warning 'Store identity is not configured; preparing portable downloads only.'
}

Get-ChildItem $dist -File | Where-Object { $_.Extension -in '.exe', '.msixbundle' } | Sort-Object Name | ForEach-Object {
    '{0}  {1}' -f (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $_.Name
} | Set-Content "$dist/SHA256SUMS.txt" -Encoding ascii
@(
    '<details>'
    '<summary>License notices</summary>'
    ''
    '```text'
    (Get-Content "$root/LICENSE" -Raw)
    '```'
    ''
    '```text'
    (Get-Content $goLicense -Raw)
    '```'
    ''
    '</details>'
) -join "`n" | Set-Content "$dist/notes.md" -Encoding utf8
Remove-Item "$dist/.build" -Recurse -Force
