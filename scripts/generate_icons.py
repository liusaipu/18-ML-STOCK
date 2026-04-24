#!/usr/bin/env python3
"""
Generate app icons for Windows (.ico) and macOS (.icns / AppIcon.iconset)
from a source image, with design enhancements.
"""
import os
import sys
from PIL import Image, ImageDraw, ImageFilter, ImageEnhance

# Paths
PROJECT_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC_IMG = os.path.join(PROJECT_ROOT, "assets", "icons", "source", "red-w.png")
OUT_DIR = os.path.join(PROJECT_ROOT, "assets", "icons", "generated")
WIN_ICON = os.path.join(PROJECT_ROOT, "build", "windows", "icon.ico")
DARWIN_DIR = os.path.join(PROJECT_ROOT, "build", "darwin", "AppIcon.iconset")
APPICON = os.path.join(PROJECT_ROOT, "build", "appicon.png")

# Ensure output dirs exist
os.makedirs(OUT_DIR, exist_ok=True)
os.makedirs(DARWIN_DIR, exist_ok=True)
os.makedirs(os.path.dirname(WIN_ICON), exist_ok=True)

# Icon sizes
WINDOWS_SIZES = [16, 20, 24, 32, 40, 48, 64, 96, 128, 256, 512]
MACOS_SIZES = [16, 32, 64, 128, 256, 512, 1024]


def rounded_rect_mask(size, radius_ratio=0.22):
    """Create a rounded rectangle mask for macOS-style icons."""
    w, h = size
    radius = int(min(w, h) * radius_ratio)
    mask = Image.new("L", size, 0)
    draw = ImageDraw.Draw(mask)
    draw.rounded_rectangle([0, 0, w - 1, h - 1], radius=radius, fill=255)
    return mask


def make_gradient(size, color_top, color_bottom):
    """Create a vertical gradient image."""
    w, h = size
    base = Image.new("RGBA", size, color_top)
    draw = ImageDraw.Draw(base)
    for y in range(h):
        ratio = y / (h - 1) if h > 1 else 0
        r = int(color_top[0] * (1 - ratio) + color_bottom[0] * ratio)
        g = int(color_top[1] * (1 - ratio) + color_bottom[1] * ratio)
        b = int(color_top[2] * (1 - ratio) + color_bottom[2] * ratio)
        a = int(color_top[3] * (1 - ratio) + color_bottom[3] * ratio)
        draw.line([(0, y), (w - 1, y)], fill=(r, g, b, a))
    return base


def remove_white_bg_smart(img, threshold=210):
    """Remove white/light background and clean edge halos."""
    pixels = img.load()
    width, height = img.size
    for y in range(height):
        for x in range(width):
            r, g, b, a = pixels[x, y]
            # Pure white -> transparent
            if r >= 245 and g >= 245 and b >= 245:
                pixels[x, y] = (r, g, b, 0)
                continue
            # Calculate brightness and colorfulness
            whiteness = max(r, g, b)
            min_val = min(r, g, b)
            color_range = max(r, g, b) - min_val
            # If very bright and not very colorful, it's background fringe
            if whiteness > threshold and color_range < 80:
                fade = max(0, int(255 * (1.0 - (whiteness - threshold) / (255 - threshold))))
                pixels[x, y] = (r, g, b, min(a, fade))
            elif whiteness > 230 and color_range < 50:
                # Aggressive fade for gray-ish bright pixels
                fade = max(0, int(255 * (1.0 - (whiteness - 200) / 55)))
                pixels[x, y] = (r, g, b, min(a, fade))
    
    # Second pass: edge halo cleanup
    # Find pixels that are semi-transparent but have high brightness -> likely halo
    for y in range(height):
        for x in range(width):
            r, g, b, a = pixels[x, y]
            if 10 < a < 180:
                brightness = (r + g + b) / 3
                # If a semi-transparent pixel is bright, reduce alpha more
                if brightness > 180:
                    new_a = int(a * 0.3)
                    pixels[x, y] = (r, g, b, new_a)
                elif brightness > 150:
                    new_a = int(a * 0.6)
                    pixels[x, y] = (r, g, b, new_a)
    return img


def prepare_source():
    """Load source, upscale, remove bg, enhance colors."""
    src = Image.open(SRC_IMG).convert("RGBA")
    
    # Upscale first to get smooth edges (source is only 195x147)
    upscale = 6
    src = src.resize((src.width * upscale, src.height * upscale), Image.LANCZOS)
    
    # Remove white background
    src = remove_white_bg_smart(src, threshold=210)
    
    # Enhance: make reds more vibrant and add contrast
    enhancer = ImageEnhance.Contrast(src)
    src = enhancer.enhance(1.25)
    enhancer = ImageEnhance.Sharpness(src)
    src = enhancer.enhance(1.4)
    enhancer = ImageEnhance.Color(src)
    src = enhancer.enhance(1.3)
    
    return src


def add_drop_shadow(img, offset=(0, 10), blur=18, shadow_color=(0, 0, 0, 110)):
    """Add a soft drop shadow beneath an RGBA image."""
    w, h = img.size
    pad = blur * 2
    shadow = Image.new("RGBA", (w + pad * 2, h + pad * 2), (0, 0, 0, 0))
    alpha = img.split()[3]
    shadow.paste(shadow_color, (pad, pad), alpha)
    shadow = shadow.filter(ImageFilter.GaussianBlur(blur / 2))
    result = Image.new("RGBA", shadow.size, (0, 0, 0, 0))
    result.paste(shadow, (0, 0), shadow)
    result.paste(img, (pad - offset[0], pad - offset[1]), img)
    return result


def add_outer_glow(img, blur=40, glow_color=(220, 30, 40, 70)):
    """Add a subtle red outer glow behind the image."""
    w, h = img.size
    pad = blur * 2
    glow = Image.new("RGBA", (w + pad * 2, h + pad * 2), (0, 0, 0, 0))
    alpha = img.split()[3]
    glow.paste(glow_color, (pad, pad), alpha)
    glow = glow.filter(ImageFilter.GaussianBlur(blur / 2))
    result = Image.new("RGBA", glow.size, (0, 0, 0, 0))
    result.paste(glow, (0, 0), glow)
    result.paste(img, (pad, pad), img)
    return result


def add_inner_highlight(bg_img, radius_ratio=0.22, opacity=22):
    """Add a subtle top inner highlight to a rounded rect background."""
    w, h = bg_img.size
    radius = int(min(w, h) * radius_ratio)
    overlay = Image.new("RGBA", (w, h), (255, 255, 255, 0))
    draw = ImageDraw.Draw(overlay)
    highlight_height = max(2, int(h * 0.12))
    for y in range(highlight_height):
        alpha = int(opacity * (1 - y / highlight_height))
        draw.rounded_rectangle(
            [3, 3 + y, w - 4, h - 4],
            radius=max(0, radius - 3),
            fill=(255, 255, 255, alpha)
        )
    result = Image.alpha_composite(bg_img, overlay)
    return result


def create_bg(size, for_macos=False):
    """Create a beautiful background with gradient and rounded corners."""
    w, h = size
    rr = 0.22 if for_macos else 0.18
    
    # Rich dark gradient with subtle red undertone
    bg_top = (35, 32, 36, 255)
    bg_bottom = (20, 18, 22, 255)
    bg = make_gradient((w, h), bg_top, bg_bottom)
    
    # Apply rounded mask
    mask = rounded_rect_mask((w, h), radius_ratio=rr)
    bg.putalpha(mask)
    
    # Inner highlight
    bg = add_inner_highlight(bg, radius_ratio=rr, opacity=25)
    
    return bg


def generate_icon_design(size, for_macos=False, src=None):
    """Generate a single enhanced icon at the given size."""
    if src is None:
        src = prepare_source()
    
    w = h = size
    padding = int(size * 0.12)  # smaller padding = bigger W
    target_w = w - padding * 2
    target_h = h - padding * 2
    
    # Preserve aspect ratio
    src_ratio = src.width / src.height
    if target_w / target_h > src_ratio:
        new_h = target_h
        new_w = int(new_h * src_ratio)
    else:
        new_w = target_w
        new_h = int(new_w / src_ratio)
    
    # Resize to target
    w_img = src.resize((new_w, new_h), Image.LANCZOS)
    
    # Layer effects: glow -> shadow
    w_glow = add_outer_glow(w_img, blur=max(10, size // 25), glow_color=(210, 35, 45, 65))
    w_final = add_drop_shadow(w_glow, offset=(0, max(2, size // 70)), blur=max(8, size // 30), shadow_color=(0, 0, 0, 90))
    
    # Background
    bg = create_bg((w, h), for_macos=for_macos)
    
    # For macOS, add subtle outer shadow to the whole icon
    if for_macos and size >= 64:
        shadow_pad = max(6, size // 70)
        blur_r = max(6, size // 50)
        full_shadow = Image.new("RGBA", (w + shadow_pad * 2, h + shadow_pad * 2), (0, 0, 0, 0))
        full_shadow.paste((0, 0, 0, 35), (shadow_pad, shadow_pad + max(2, size // 150)), rounded_rect_mask((w, h), radius_ratio=0.22))
        full_shadow = full_shadow.filter(ImageFilter.GaussianBlur(blur_r))
        
        canvas = Image.new("RGBA", full_shadow.size, (0, 0, 0, 0))
        canvas.paste(full_shadow, (0, 0), full_shadow)
        canvas.paste(bg, (shadow_pad, shadow_pad), bg)
        
        # Center W
        wx, wy = w_final.size
        cx = (canvas.width - wx) // 2
        cy = (canvas.height - wy) // 2 - max(1, size // 150)
        canvas.paste(w_final, (cx, cy), w_final)
        return canvas
    else:
        final = Image.new("RGBA", (w, h), (0, 0, 0, 0))
        final.paste(bg, (0, 0), bg)
        wx, wy = w_final.size
        cx = (w - wx) // 2
        cy = (h - wy) // 2 - max(1, size // 150)
        final.paste(w_final, (cx, cy), w_final)
        return final


def generate_macos_iconset():
    """Generate macOS AppIcon.iconset folder."""
    print("Generating macOS AppIcon.iconset...")
    src = prepare_source()
    for size in MACOS_SIZES:
        img = generate_icon_design(size, for_macos=True, src=src)
        path = os.path.join(DARWIN_DIR, f"icon_{size}x{size}.png")
        img.save(path, "PNG")
        if size < 1024:
            path2x = os.path.join(DARWIN_DIR, f"icon_{size}x{size}@2x.png")
            img2x = generate_icon_design(size * 2, for_macos=True, src=src)
            img2x.save(path2x, "PNG")
    print(f"  -> {DARWIN_DIR}")


def generate_windows_ico():
    """Generate Windows .ico file with multiple resolutions.
    PIL's ICO writer is flaky with multi-frame, so we manually assemble.
    """
    print("Generating Windows icon.ico...")
    src = prepare_source()
    images = []
    for size in WINDOWS_SIZES:
        img = generate_icon_design(size, for_macos=False, src=src)
        images.append(img)
    
    import struct
    from io import BytesIO
    
    # ICONDIR
    icondir = struct.pack('<HHH', 0, 1, len(images))
    
    # Calculate offsets
    header_size = 6 + 16 * len(images)
    entries = []
    png_datas = []
    offset = header_size
    
    for img in images:
        buf = BytesIO()
        img.save(buf, format='PNG')
        data = buf.getvalue()
        png_datas.append(data)
        w, h = img.size
        # ICO directory entry
        entries.append(struct.pack(
            '<BBBBHHII',
            w if w < 256 else 0,      # width
            h if h < 256 else 0,      # height
            0,                          # colors (0 for PNG)
            0,                          # reserved
            1,                          # planes
            32,                         # bit depth
            len(data),                  # size in bytes
            offset                      # offset
        ))
        offset += len(data)
    
    with open(WIN_ICON, 'wb') as f:
        f.write(icondir)
        for entry in entries:
            f.write(entry)
        for data in png_datas:
            f.write(data)
    
    print(f"  -> {WIN_ICON}")


def generate_appicon():
    """Generate the main appicon.png for Wails."""
    print("Generating build/appicon.png...")
    src = prepare_source()
    img = generate_icon_design(1024, for_macos=True, src=src)
    img.save(APPICON, "PNG")
    print(f"  -> {APPICON}")


def main():
    if not os.path.exists(SRC_IMG):
        print(f"Source image not found: {SRC_IMG}")
        sys.exit(1)
    
    print(f"Source: {SRC_IMG}")
    generate_macos_iconset()
    generate_windows_ico()
    generate_appicon()
    print("\nDone! All icons generated.")


if __name__ == "__main__":
    main()
