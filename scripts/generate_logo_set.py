#!/usr/bin/env python3
"""
Generate a complete icon/logo set from red-w.png.
W keeps its original line style & color gradient.
Background: deep blue radial gradient (center light -> edge dark).
"""
import os
from PIL import Image, ImageDraw, ImageFilter, ImageEnhance

#!/usr/bin/env python3
"""
Generate a complete icon/logo set from master source images.

Sources:
  - assets/new-res_1024.png  → sizes >= 64
  - assets/new-res_32.png    → sizes 16, 32
"""
import os
from PIL import Image

PROJECT_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC_1024 = os.path.join(PROJECT_ROOT, "assets", "new-res_1024.png")
SRC_32   = os.path.join(PROJECT_ROOT, "assets", "new-res_32.png")
OUT      = os.path.join(PROJECT_ROOT, "assets", "icons", "generated")
os.makedirs(OUT, exist_ok=True)


def get_source_for_size(size):
    """Return the best source image for a given output size."""
    if size <= 32:
        return Image.open(SRC_32).convert("RGBA")
    return Image.open(SRC_1024).convert("RGBA")


def resize_icon(size):
    """Resize from the appropriate source, preserving quality."""
    src = get_source_for_size(size)
    # Use NEAREST for tiny sizes to keep crisp edges, LANCZOS for larger
    method = Image.NEAREST if size <= 16 else Image.LANCZOS
    return src.resize((size, size), method)


import struct
import io

def make_ico(sizes, out_path):
    """Create a multi-frame ICO file (PNG format per frame, supports alpha)."""
    # ICO standard sizes only: 16,32,48,64,128,256
    ico_sizes = [sz for sz in sizes if sz in (16, 32, 48, 64, 128, 256)]
    frames = [resize_icon(sz).convert("RGBA") for sz in ico_sizes]

    # Build ICO header
    count = len(frames)
    header = struct.pack('<HHH', 0, 1, count)  # reserved, type=icon, count

    entries = b''
    data_offset = 6 + 16 * count
    image_data = b''

    for idx, (img, sz) in enumerate(zip(frames, ico_sizes)):
        # Save each frame as PNG
        buf = io.BytesIO()
        img.save(buf, format='PNG')
        png_bytes = buf.getvalue()

        # ICONDIRENTRY
        w_byte = sz if sz < 256 else 0
        h_byte = sz if sz < 256 else 0
        entry = struct.pack('<BBBBHHII',
            w_byte, h_byte,  # width, height
            0,               # colors (0 = >256)
            0,               # reserved
            1,               # color planes
            32,              # bits per pixel
            len(png_bytes),  # size in bytes
            data_offset      # offset
        )
        entries += entry
        image_data += png_bytes
        data_offset += len(png_bytes)

    with open(out_path, 'wb') as f:
        f.write(header + entries + image_data)


def main():
    # ── 1. App icon master (1024) ──────────────────────────
    master = resize_icon(1024)
    master.save(os.path.join(OUT, "appicon-master.png"))
    print("→ appicon-master.png (1024)")

    # ── 2. Windows icon ────────────────────────────────────
    win_sizes = [16, 20, 24, 32, 40, 48, 64, 96, 128, 256]
    make_ico(win_sizes, os.path.join(OUT, "icon-windows.ico"))
    print("→ icon-windows.ico")

    # ── 3. macOS iconset ───────────────────────────────────
    mac_dir = os.path.join(OUT, "AppIcon.iconset")
    os.makedirs(mac_dir, exist_ok=True)
    mac_sizes = {
        16:  [16, 32],
        32:  [32, 64],
        64:  [64, 128],
        128: [128, 256],
        256: [256, 512],
        512: [512, 1024],
        1024:[1024],
    }
    for base, outputs in mac_sizes.items():
        for sz in outputs:
            if sz == 1024:
                fname = f"icon_{base}x{base}.png"
            elif sz == base * 2:
                fname = f"icon_{base}x{base}@2x.png"
            else:
                fname = f"icon_{sz}x{sz}.png"
            resize_icon(sz).save(os.path.join(mac_dir, fname))
    print("→ AppIcon.iconset/")

    # ── 4. Favicon set ─────────────────────────────────────
    fav_sizes = [16, 32, 48, 64, 128, 256]
    for sz in fav_sizes:
        resize_icon(sz).save(os.path.join(OUT, f"favicon-{sz}x{sz}.png"))
    make_ico(fav_sizes, os.path.join(OUT, "favicon.ico"))
    print("→ favicon set (16~256 + .ico)")

    # ── 5. Preview strip ───────────────────────────────────
    strip = Image.new("RGBA", (640, 160), (30, 35, 50, 255))
    previews = [resize_icon(sz) for sz in [128, 64, 48, 32]]
    x = 20
    for p in previews:
        strip.paste(p, (x, 16), p)
        x += 155
    strip.save(os.path.join(OUT, "preview-strip.png"))
    print("→ preview-strip.png")

    # ── 6. Copy to build/ directories ──────────────────────
    # appicon.png
    resize_icon(1024).save(os.path.join(PROJECT_ROOT, "build", "appicon.png"))
    # Windows icon
    make_ico(win_sizes, os.path.join(PROJECT_ROOT, "build", "windows", "icon.ico"))
    # macOS iconset
    build_mac_dir = os.path.join(PROJECT_ROOT, "build", "darwin", "AppIcon.iconset")
    os.makedirs(build_mac_dir, exist_ok=True)
    for base, outputs in mac_sizes.items():
        for sz in outputs:
            if sz == 1024:
                fname = f"icon_{base}x{base}.png"
            elif sz == base * 2:
                fname = f"icon_{base}x{base}@2x.png"
            else:
                fname = f"icon_{sz}x{sz}.png"
            resize_icon(sz).save(os.path.join(build_mac_dir, fname))
    print("→ synced to build/")

    print("\nAll done! Sources: new-res_32.png (≤32), new-res_1024.png (≥64)")


if __name__ == "__main__":
    main()
