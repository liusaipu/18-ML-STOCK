#!/usr/bin/env python3
"""
Generate a complete icon/logo set from red-w.png.
W keeps its original line style & color gradient.
Background: deep blue radial gradient (center light -> edge dark).
"""
import os
from PIL import Image, ImageDraw, ImageFilter, ImageEnhance

PROJECT_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(PROJECT_ROOT, "assets", "icons", "source", "red-w.png")
OUT = os.path.join(PROJECT_ROOT, "assets", "icons", "generated")
os.makedirs(OUT, exist_ok=True)

# ── helpers ──────────────────────────────────────────────
def rounded_rect_mask(size, rr=0.22):
    w, h = size
    r = int(min(w, h) * rr)
    m = Image.new("L", size, 0)
    ImageDraw.Draw(m).rounded_rectangle([0, 0, w - 1, h - 1], radius=r, fill=255)
    return m

def radial_gradient(size, c_center, c_edge):
    w, h = size
    cx, cy = w / 2, h / 2
    max_d = ((cx) ** 2 + (cy) ** 2) ** 0.5
    img = Image.new("RGBA", size)
    px = img.load()
    for y in range(h):
        for x in range(w):
            d = ((x - cx) ** 2 + (y - cy) ** 2) ** 0.5
            t = min(1.0, d / max_d)
            r = int(c_center[0] * (1 - t) + c_edge[0] * t)
            g = int(c_center[1] * (1 - t) + c_edge[1] * t)
            b = int(c_center[2] * (1 - t) + c_edge[2] * t)
            a = int(c_center[3] * (1 - t) + c_edge[3] * t)
            px[x, y] = (r, g, b, a)
    return img

def inner_highlight(bg, rr=0.22, opacity=28):
    w, h = bg.size
    r = int(min(w, h) * rr)
    ov = Image.new("RGBA", (w, h), (255, 255, 255, 0))
    dr = ImageDraw.Draw(ov)
    hh = max(2, int(h * 0.12))
    for y in range(hh):
        a = int(opacity * (1 - y / hh))
        dr.rounded_rectangle([3, 3 + y, w - 4, h - 4], radius=max(0, r - 3), fill=(255, 255, 255, a))
    return Image.alpha_composite(bg, ov)

def remove_bg(img, thr=210):
    """Smart background removal that preserves red W edges."""
    px = img.load()
    w, h = img.size
    for y in range(h):
        for x in range(w):
            r, g, b, a = px[x, y]
            # Pure white -> transparent
            if r >= 245 and g >= 245 and b >= 245:
                px[x, y] = (r, g, b, 0)
                continue
            bright = max(r, g, b)
            min_val = min(r, g, b)
            color_range = bright - min_val
            # If it's bright AND reddish (R dominant), keep it as W edge
            is_red = r > g + 20 and r > b + 10
            if bright > thr and not is_red:
                # Smooth fade for non-red bright pixels (background fringe)
                fade = max(0, int(255 * (1.0 - (bright - thr) / (255 - thr))))
                px[x, y] = (r, g, b, min(a, fade))
            elif bright > 230 and color_range < 40:
                # Aggressive fade for gray-ish bright pixels
                fade = max(0, int(255 * (1.0 - (bright - 200) / 55)))
                px[x, y] = (r, g, b, min(a, fade))
    # Second pass: clean remaining semi-transparent bright halos
    for y in range(h):
        for x in range(w):
            r, g, b, a = px[x, y]
            if 10 < a < 180:
                br = (r + g + b) / 3
                is_red = r > g + 15 and r > b + 5
                if br > 180 and not is_red:
                    px[x, y] = (r, g, b, int(a * 0.3))
                elif br > 150 and not is_red:
                    px[x, y] = (r, g, b, int(a * 0.6))
    return img

def prepare_w():
    """Load W, upscale, remove bg, thicken strokes, brighten colors."""
    src = Image.open(SRC).convert("RGBA")
    src = src.resize((src.width * 10, src.height * 10), Image.LANCZOS)
    src = remove_bg(src, thr=210)
    
    # Thicken strokes by dilating alpha
    r, g, b, a = src.split()
    a = a.filter(ImageFilter.MaxFilter(5))
    src = Image.merge("RGBA", (r, g, b, a))
    
    # Brighten & enhance while keeping original gradient
    src = ImageEnhance.Brightness(src).enhance(1.15)
    src = ImageEnhance.Contrast(src).enhance(1.2)
    src = ImageEnhance.Sharpness(src).enhance(1.3)
    src = ImageEnhance.Color(src).enhance(1.3)
    return src

def make_app_icon(size, w_src):
    """Round-rect app icon with deep-blue radial bg."""
    w = h = size
    # deep blue radial: center lighter, edge darker
    bg = radial_gradient(
        (w, h),
        c_center=(45, 70, 135, 255),   # lighter deep blue
        c_edge=(8, 14, 35, 255)         # darker deep blue
    )
    mask = rounded_rect_mask((w, h), rr=0.22 if size >= 128 else 0.18)
    bg.putalpha(mask)
    bg = inner_highlight(bg, rr=0.22 if size >= 128 else 0.18, opacity=28)

    # Place W
    pad = int(size * 0.08)
    tw = w - pad * 2
    th = h - pad * 2
    ratio = w_src.width / w_src.height
    if tw / th > ratio:
        nh = th
        nw = int(nh * ratio)
    else:
        nw = tw
        nh = int(nw / ratio)

    w_img = w_src.resize((nw, nh), Image.LANCZOS)

    # Glow + shadow
    glow_pad = max(8, size // 30)
    glow = Image.new("RGBA", (nw + glow_pad * 2, nh + glow_pad * 2), (0, 0, 0, 0))
    glow.paste((220, 40, 50, 60), (glow_pad, glow_pad), w_img.split()[3])
    glow = glow.filter(ImageFilter.GaussianBlur(glow_pad / 2))

    shadow_pad = max(6, size // 40)
    shadow = Image.new("RGBA", (nw + shadow_pad * 2, nh + shadow_pad * 2), (0, 0, 0, 0))
    shadow.paste((0, 0, 0, 80), (shadow_pad, shadow_pad + max(2, size // 150)), w_img.split()[3])
    shadow = shadow.filter(ImageFilter.GaussianBlur(shadow_pad / 2))

    # Composite
    fx = Image.new("RGBA", shadow.size, (0, 0, 0, 0))
    fx.paste(shadow, (0, 0), shadow)
    fx.paste(glow, (0, 0), glow)
    fx.paste(w_img, (shadow_pad, shadow_pad), w_img)

    # Center on bg
    final = Image.new("RGBA", (w, h), (0, 0, 0, 0))
    final.paste(bg, (0, 0), bg)
    cx = (w - fx.width) // 2
    cy = (h - fx.height) // 2 - max(1, size // 150)
    final.paste(fx, (cx, cy), fx)
    return final

def make_logo_on_transparent(w_src, size=1024):
    """Pure W on transparent background (for README / docs)."""
    ratio = w_src.width / w_src.height
    nw = size
    nh = int(nw / ratio)
    img = w_src.resize((nw, nh), Image.LANCZOS)
    # subtle glow for visibility on both light & dark backgrounds
    pad = 60
    glow = Image.new("RGBA", (nw + pad * 2, nh + pad * 2), (0, 0, 0, 0))
    glow.paste((220, 40, 50, 45), (pad, pad), img.split()[3])
    glow = glow.filter(ImageFilter.GaussianBlur(pad / 3))
    canvas = Image.new("RGBA", glow.size, (0, 0, 0, 0))
    canvas.paste(glow, (0, 0), glow)
    canvas.paste(img, (pad, pad), img)
    return canvas

def make_social_banner(w_src, size=(1280, 640)):
    """Wide banner for GitHub social preview / OG image."""
    w, h = size
    bg = radial_gradient(size, c_center=(42, 65, 125, 255), c_edge=(6, 10, 25, 255))
    # Place W on left side
    ratio = w_src.width / w_src.height
    nh = int(h * 0.65)
    nw = int(nh * ratio)
    w_img = w_src.resize((nw, nh), Image.LANCZOS)
    # glow
    pad = 40
    glow = Image.new("RGBA", (nw + pad * 2, nh + pad * 2), (0, 0, 0, 0))
    glow.paste((220, 40, 50, 55), (pad, pad), w_img.split()[3])
    glow = glow.filter(ImageFilter.GaussianBlur(pad / 2))
    # shadow
    sp = 30
    shadow = Image.new("RGBA", (nw + sp * 2, nh + sp * 2), (0, 0, 0, 0))
    shadow.paste((0, 0, 0, 70), (sp, sp + 4), w_img.split()[3])
    shadow = shadow.filter(ImageFilter.GaussianBlur(sp / 2))
    # composite
    fx = Image.new("RGBA", shadow.size, (0, 0, 0, 0))
    fx.paste(shadow, (0, 0), shadow)
    fx.paste(glow, (0, 0), glow)
    fx.paste(w_img, (sp, sp), w_img)
    # place
    bg.paste(fx, (int(w * 0.08), (h - fx.height) // 2), fx)
    # Add text-like brand placeholder (we keep it icon-only for now)
    return bg

def make_favicon(w_src):
    """Multi-size favicon set."""
    sizes = [16, 32, 48, 64, 128, 256]
    for sz in sizes:
        icon = make_app_icon(sz, w_src)
        icon.save(os.path.join(OUT, f"favicon-{sz}x{sz}.png"))
    # also ico
    ico_frames = [make_app_icon(sz, w_src).convert("RGBA") for sz in sizes]
    ico_frames[0].save(os.path.join(OUT, "favicon.ico"), format="ICO", append_images=ico_frames[1:])

def main():
    w = prepare_w()

    # 1. App icon master
    app = make_app_icon(1024, w)
    app.save(os.path.join(OUT, "appicon-master.png"))
    print("→ appicon-master.png (1024)")

    # 2. Logo on transparent
    logo = make_logo_on_transparent(w, 1024)
    logo.save(os.path.join(OUT, "logo-transparent.png"))
    print("→ logo-transparent.png")

    # 3. Social / OG banner
    banner = make_social_banner(w)
    banner.save(os.path.join(OUT, "social-banner.png"))
    print("→ social-banner.png (1280x640)")

    # 4. Favicon set
    make_favicon(w)
    print("→ favicon set (16~256 + .ico)")

    # 5. Windows icon
    win_sizes = [16, 20, 24, 32, 40, 48, 64, 96, 128, 256, 512]
    win_frames = [make_app_icon(sz, w).convert("RGBA") for sz in win_sizes]
    win_frames[0].save(os.path.join(OUT, "icon-windows.ico"), format="ICO", append_images=win_frames[1:])
    print("→ icon-windows.ico")

    # 6. macOS iconset
    mac_dir = os.path.join(OUT, "AppIcon.iconset")
    os.makedirs(mac_dir, exist_ok=True)
    for sz in [16, 32, 64, 128, 256, 512, 1024]:
        make_app_icon(sz, w).save(os.path.join(mac_dir, f"icon_{sz}x{sz}.png"))
        if sz < 1024:
            make_app_icon(sz * 2, w).save(os.path.join(mac_dir, f"icon_{sz}x{sz}@2x.png"))
    print("→ AppIcon.iconset/")

    # 7. Preview strip for user review
    from PIL import Image as PILImage
    strip = PILImage.new("RGBA", (640, 160), (30, 35, 50, 255))
    previews = [
        make_app_icon(128, w),
        make_app_icon(64, w),
        make_app_icon(48, w),
        make_app_icon(32, w),
    ]
    x = 20
    for p in previews:
        strip.paste(p, (x, 16), p)
        x += 155
    strip.save(os.path.join(OUT, "preview-strip.png"))
    print("→ preview-strip.png")

    print("\nAll done! Check assets/icons/generated/")

if __name__ == "__main__":
    main()
