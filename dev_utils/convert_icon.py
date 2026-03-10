#!/usr/bin/env python3
"""
Convert PNG to ICO for Windows application icon and copy as appicon
"""
from PIL import Image
import sys
import shutil

def convert_png_to_ico(png_path, appicon_path, ico_path):
    """Convert PNG image to ICO format with multiple sizes and copy as appicon"""
    try:
        # Copy as appicon
        print(f"Copying {png_path} to {appicon_path}...")
        shutil.copy2(png_path, appicon_path)
        
        # Open the PNG image
        img = Image.open(png_path)
        
        # Convert to RGBA if not already
        if img.mode != 'RGBA':
            img = img.convert('RGBA')
        
        # Create icons in multiple sizes (standard Windows icon sizes)
        icon_sizes = [(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
        
        print(f"Converting to Windows ICO format...")
        # Save as ICO with multiple sizes
        img.save(ico_path, format='ICO', sizes=icon_sizes)
        print(f"\n✓ Icon generation complete!")
        print(f"  App Icon: {appicon_path}")
        print(f"  Windows Icon: {ico_path}")
        print(f"  Icon sizes: {', '.join([f'{w}x{h}' for w, h in icon_sizes])}")
        return True
    except ImportError:
        print("ERROR: Pillow (PIL) is not installed.")
        print("Install it with: pip install Pillow")
        return False
    except Exception as e:
        print(f"ERROR: Failed to convert image: {e}")
        return False

if __name__ == "__main__":
    png_file = "../logo_jo26_spiele.png"
    appicon_file = "../build/appicon.png"
    ico_file = "../build/windows/icon.ico"
    
    print(f"Processing application icon from {png_file}...")
    if convert_png_to_ico(png_file, appicon_file, ico_file):
        sys.exit(0)
    else:
        sys.exit(1)
