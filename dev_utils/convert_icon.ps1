# PowerShell script to convert PNG to ICO and copy as appicon
# This uses .NET System.Drawing to create a Windows icon

param(
    [string]$PngPath = "..\logo_jo26_spiele.png",
    [string]$AppIconPath = "..\build\appicon.png",
    [string]$IcoPath = "..\build\windows\icon.ico"
)

Write-Host "Processing application icon..." -ForegroundColor Cyan

# Copy PNG as appicon
Write-Host "Copying $PngPath to $AppIconPath..." -ForegroundColor Gray
Copy-Item -Path $PngPath -Destination $AppIconPath -Force

Write-Host "Converting to Windows ICO format..." -ForegroundColor Gray

try {
    # Load required assemblies
    Add-Type -AssemblyName System.Drawing
    
    # Load the PNG image
    $pngFullPath = (Resolve-Path $PngPath).Path
    $png = [System.Drawing.Image]::FromFile($pngFullPath)
    
    # Create a bitmap at 256x256
    $bitmap = New-Object System.Drawing.Bitmap($png, 256, 256)
    
    # Get icon handle
    $iconHandle = $bitmap.GetHicon()
    $icon = [System.Drawing.Icon]::FromHandle($iconHandle)
    
    # Save as ICO file
    $icoFullPath = Join-Path (Get-Location) $IcoPath
    $fileStream = [System.IO.File]::Create($icoFullPath)
    $icon.Save($fileStream)
    $fileStream.Close()
    
    # Clean up
    $icon.Dispose()
    $bitmap.Dispose()
    $png.Dispose()
    
    Write-Host "`n✓ Icon generation complete!" -ForegroundColor Green
    Write-Host "  App Icon: $AppIconPath" -ForegroundColor Gray
    Write-Host "  Windows Icon: $IcoPath (256x256)" -ForegroundColor Gray
    
} catch {
    Write-Host "ERROR: Failed to convert image" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
    exit 1
}
