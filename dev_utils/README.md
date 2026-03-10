# Development Utilities

This directory contains utility scripts for development and build tasks.

## Icon Generator

Generate application icons from the logo file.

### Source File
- **Logo**: `../logo_jo26_spiele.png` (root directory)

### Generated Files
- **App Icon (PNG)**: `../build/appicon.png` - Used by Wails for various platforms
- **Windows Icon**: `../build/windows/icon.ico` - Used for Windows executable

### How to Regenerate Icons

#### Method 1: PowerShell (Windows - Recommended)
```powershell
cd dev_utils
powershell -ExecutionPolicy Bypass -File convert_icon.ps1
```

Or from project root:
```powershell
powershell -ExecutionPolicy Bypass -File dev_utils\convert_icon.ps1
```

This script will:
1. Copy `logo_jo26_spiele.png` → `build/appicon.png`
2. Convert PNG to ICO format at 256x256 resolution → `build/windows/icon.ico`

#### Method 2: Python (Cross-platform)
If you have Python with Pillow installed:
```bash
cd dev_utils
pip install Pillow
python convert_icon.py
```

Or from project root:
```bash
python dev_utils\convert_icon.py
```

This script will:
1. Copy `logo_jo26_spiele.png` → `build/appicon.png`
2. Convert PNG to ICO with multiple sizes (16x16, 32x32, 48x48, 64x64, 128x128, 256x256) → `build/windows/icon.ico`

### After Icon Update

Rebuild the application to see the new icon:
```bash
wails build
```

The icon will appear on:
- Windows executable (.exe)
- Application window title bar
- Taskbar
- Desktop shortcuts

### Icon Specifications

- **Format**: PNG source, ICO for Windows
- **Minimum size**: 256x256 pixels
- **Recommended**: Square aspect ratio, transparent background
- **Color mode**: RGBA (supports transparency)

### Notes

- The PowerShell script uses .NET System.Drawing
- The Python script requires the Pillow library
- Wails automatically handles icon conversion for some platforms
- Always keep the original high-resolution PNG file
