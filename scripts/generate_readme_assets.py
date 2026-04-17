#!/usr/bin/env python3
"""Generate README image assets for StockFinLens"""
import os
import math
import random

from PIL import Image, ImageDraw, ImageFont

random.seed(42)

# Ensure output dirs exist
os.makedirs("docs/screenshots", exist_ok=True)
os.makedirs("docs", exist_ok=True)

# ── Color palette (dark theme) ──
def hex2rgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i:i+2], 16) for i in (0, 2, 4))

BG = "#0d1117"
CARD = "#161b22"
BORDER = "#30363d"
TEXT_PRIMARY = "#e6edf3"
TEXT_SECONDARY = "#8b949e"
GREEN = "#3fb950"
RED = "#f85149"
YELLOW = "#d29922"
BLUE = "#58a6ff"
CYAN = "#39d0d8"
GREEN_RGB = hex2rgb(GREEN)
RED_RGB = hex2rgb(RED)


def get_font(size):
    """Try common fonts, fallback to default."""
    candidates = [
        "C:\\Windows\\Fonts\\msyh.ttc",        # Microsoft YaHei
        "C:\\Windows\\Fonts\\msyhbd.ttc",
        "C:\\Windows\\Fonts\\segoeui.ttf",
        "C:\\Windows\\Fonts\\arial.ttf",
    ]
    for path in candidates:
        if os.path.exists(path):
            try:
                return ImageFont.truetype(path, size)
            except Exception:
                continue
    return ImageFont.load_default()


# ═══════════════════════════════════════════════
# 1. Logo
# ═══════════════════════════════════════════════
def draw_logo():
    W, H = 512, 640
    img = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)

    # Dark rounded background for visibility on any theme
    d.rounded_rectangle([0, 0, W, H], radius=60, fill=(13, 17, 23, 255))

    cx, cy = W // 2, 260  # lens center shifted up to make room for text

    # Outer ring (subtle glow)
    for r in range(180, 200, 2):
        alpha = int(30 * (1 - (r - 180) / 20))
        d.ellipse([cx - r, cy - r, cx + r, cy + r], outline=(88, 166, 255, alpha), width=2)

    # Main circle (lens body)
    r_main = 170
    d.ellipse([cx - r_main, cy - r_main, cx + r_main, cy + r_main],
              fill=(22, 27, 34, 255), outline=(88, 166, 255, 200), width=6)

    # Inner gradient-like ring
    r_inner = 158
    for i in range(3):
        alpha = 60 - i * 15
        d.ellipse([cx - r_inner + i*4, cy - r_inner + i*4,
                   cx + r_inner - i*4, cy + r_inner - i*4],
                  outline=(57, 208, 216, alpha), width=2)

    # Chart line inside lens: 上涨 → 小波折 → 继续上涨创新高
    points = []
    base_y = cy + 55
    x_start = cx - 95
    x_end = cx + 95
    n = 30
    for i in range(n):
        x = x_start + (x_end - x_start) * i / (n - 1)
        t = i / (n - 1)
        # 大趋势：S型曲线整体向上，终点创新高
        main_trend = (3 * t * t - 2 * t * t * t) * 85  # 0 -> 85
        # 小波折：中间叠加震荡回调
        wobble = 0
        if 0.22 < t < 0.42:
            wobble = math.sin((t - 0.22) / 0.2 * math.pi) * 20  # 回调
        elif 0.50 < t < 0.72:
            wobble = -math.sin((t - 0.50) / 0.22 * math.pi) * 14  # 二次震荡
        noise = (random.random() - 0.5) * 10
        y = base_y - main_trend + wobble + noise
        points.append((x, y))

    # Draw connecting line
    for i in range(len(points) - 1):
        d.line([points[i], points[i + 1]], fill=(57, 208, 216, 230), width=4)

    # Draw candle bodies on key points (对齐走势高低点)
    candle_indices = [3, 8, 13, 18, 24]
    for idx, ci in enumerate(candle_indices):
        x, y = points[ci]
        # 根据位置决定涨跌：前段涨、回调跌、再涨
        if idx in [0, 2, 4]:
            is_up = True
        else:
            is_up = False
        color_rgb = GREEN_RGB if is_up else RED_RGB
        body_h = random.randint(10, 22)
        wick_h = random.randint(5, 12)
        # body
        d.rectangle([x - 4, y - body_h // 2, x + 4, y + body_h // 2],
                    fill=(*color_rgb, 220), outline=(*color_rgb, 255), width=1)
        # wick
        d.line([x, y - body_h // 2 - wick_h, x, y - body_h // 2], fill=(*color_rgb, 180), width=2)
        d.line([x, y + body_h // 2, x, y + body_h // 2 + wick_h], fill=(*color_rgb, 180), width=2)

    # Magnifying glass handle
    handle_len = 65
    handle_angle = math.radians(45)
    hx1 = cx + int(r_main * math.cos(handle_angle))
    hy1 = cy + int(r_main * math.sin(handle_angle))
    hx2 = hx1 + int(handle_len * math.cos(handle_angle))
    hy2 = hy1 + int(handle_len * math.sin(handle_angle))
    d.line([(hx1, hy1), (hx2, hy2)], fill=(88, 166, 255, 230), width=12)
    d.line([(hx1, hy1), (hx2, hy2)], fill=(136, 192, 255, 150), width=5)

    # Brand text below
    font_brand = get_font(42)
    text = "StockFinLens"
    bbox = d.textbbox((0, 0), text, font=font_brand)
    tw = bbox[2] - bbox[0]
    d.text((cx - tw // 2, cy + r_main + 25), text, fill=(230, 237, 243, 255), font=font_brand)

    # Subtitle
    font_sub = get_font(22)
    sub = "财报透镜"
    bbox = d.textbbox((0, 0), sub, font=font_sub)
    tw = bbox[2] - bbox[0]
    d.text((cx - tw // 2, cy + r_main + 75), sub, fill=(139, 148, 158, 255), font=font_sub)

    img.save("docs/logo.png")
    print("[OK] Generated docs/logo.png")


# ═══════════════════════════════════════════════
# 2. K-line chart
# ═══════════════════════════════════════════════
def draw_kline():
    W, H = 1280, 720
    img = Image.new("RGBA", (W, H), BG)
    d = ImageDraw.Draw(img)

    # Title bar area
    d.rectangle([0, 0, W, 50], fill="#161b22")
    font_title = get_font(20)
    d.text((20, 12), "000858.SZ 五粮液 — K线透镜", fill=TEXT_PRIMARY, font=font_title)
    font_small = get_font(14)
    d.text((420, 16), "日线  MA5  MA10  MA20  VOL", fill=TEXT_SECONDARY, font=font_small)

    # Chart area
    chart_top = 60
    chart_bottom = 520
    chart_left = 60
    chart_right = 1180
    price_top = 200
    price_bottom = 120

    # Grid lines
    for i in range(6):
        y = chart_top + (chart_bottom - chart_top) * i // 5
        d.line([(chart_left, y), (chart_right, y)], fill=BORDER, width=1)
    for i in range(10):
        x = chart_left + (chart_right - chart_left) * i // 9
        d.line([(x, chart_top), (x, chart_bottom)], fill=BORDER, width=1)

    # Generate candle data
    n = 60
    prices = []
    base = 150
    p = base
    for i in range(n):
        change = (random.random() - 0.48) * 8
        p += change
        p = max(100, min(200, p))
        open_p = p + (random.random() - 0.5) * 4
        close_p = p + (random.random() - 0.5) * 4
        high_p = max(open_p, close_p) + random.random() * 3
        low_p = min(open_p, close_p) - random.random() * 3
        vol = random.randint(5000, 25000)
        prices.append((open_p, close_p, high_p, low_p, vol))

    # Price range
    all_prices = [v for o, c, h, l, v in prices for v in (h, l)]
    p_min, p_max = min(all_prices), max(all_prices)
    p_range = p_max - p_min

    def px(i):
        return chart_left + (chart_right - chart_left) * i // (n - 1)

    def py(price):
        return chart_bottom - int((price - p_min) / p_range * (chart_bottom - chart_top))

    candle_w = max(4, (chart_right - chart_left) // n - 4)

    # Draw candles
    ma5, ma10, ma20 = [], [], []
    for i, (o, c, h, l, v) in enumerate(prices):
        x = px(i)
        is_up = c >= o
        color_rgb = GREEN_RGB if is_up else RED_RGB
        color_str = GREEN if is_up else RED
        # wick
        d.line([(x, py(h)), (x, py(l))], fill=color_str, width=1)
        # body
        y_top = py(max(o, c))
        y_bot = py(min(o, c))
        d.rectangle([x - candle_w // 2, y_top, x + candle_w // 2, y_bot],
                    fill=color_rgb, outline=color_str, width=1)

        # MAs
        def avg(vals, window):
            if len(vals) < window:
                return None
            return sum(vals[-window:]) / window

        ma5.append(c)
        ma10.append(c)
        ma20.append(c)

    # Draw MA lines
    def draw_ma(window, color, width=2):
        pts = []
        for i in range(window - 1, len(prices)):
            vals = [prices[j][1] for j in range(i - window + 1, i + 1)]
            avg_val = sum(vals) / window
            pts.append((px(i), py(avg_val)))
        if len(pts) > 1:
            for i in range(len(pts) - 1):
                d.line([pts[i], pts[i + 1]], fill=color, width=width)

    draw_ma(5, CYAN)
    draw_ma(10, YELLOW)
    draw_ma(20, BLUE)

    # Price axis labels
    font_axis = get_font(12)
    for i in range(6):
        price = p_min + p_range * i / 5
        y = chart_bottom - (chart_bottom - chart_top) * i // 5
        d.text((chart_right + 8, int(y) - 6), f"{price:.1f}", fill=TEXT_SECONDARY, font=font_axis)

    # Volume chart
    vol_top = 540
    vol_bottom = 700
    d.line([(chart_left, vol_top), (chart_right, vol_top)], fill=BORDER, width=1)
    d.line([(chart_left, vol_bottom), (chart_right, vol_bottom)], fill=BORDER, width=1)

    max_vol = max(v for _, _, _, _, v in prices)
    for i, (o, c, h, l, v) in enumerate(prices):
        x = px(i)
        is_up = c >= o
        color_rgb = GREEN_RGB if is_up else RED_RGB
        h_bar = int(v / max_vol * (vol_bottom - vol_top))
        bar_color = (*color_rgb, 128)
        d.rectangle([x - candle_w // 2, vol_bottom - h_bar, x + candle_w // 2, vol_bottom],
                    fill=bar_color, outline=None)

    d.text((chart_left, vol_top - 20), "成交量", fill=TEXT_SECONDARY, font=font_axis)

    # Legend
    legend_y = chart_top + 10
    legend_x = chart_right - 200
    for label, color in [("MA5", CYAN), ("MA10", YELLOW), ("MA20", BLUE)]:
        d.rectangle([legend_x, legend_y, legend_x + 20, legend_y + 3], fill=color)
        d.text((legend_x + 28, legend_y - 5), label, fill=TEXT_SECONDARY, font=font_axis)
        legend_x += 60

    # Annotation callouts
    font_anno = get_font(13)
    d.rounded_rectangle([px(35) - 60, py(170) - 50, px(35) + 40, py(170) - 15], radius=4, fill=(35, 134, 54, 50), outline=GREEN)
    d.text((px(35) - 55, py(170) - 45), "突破压力位", fill=GREEN, font=font_anno)
    d.line([(px(35), py(170) - 15), (px(35), py(prices[35][2]))], fill=GREEN, width=1)

    d.rounded_rectangle([px(48) - 60, py(130) + 15, px(48) + 40, py(130) + 50], radius=4, fill=(248, 81, 73, 50), outline=RED)
    d.text((px(48) - 55, py(130) + 20), "放量下跌", fill=RED, font=font_anno)
    d.line([(px(48), py(130) + 15), (px(48), py(prices[48][3]))], fill=RED, width=1)

    img.save("docs/screenshots/kline.png")
    print("[OK] Generated docs/screenshots/kline.png")


# ═══════════════════════════════════════════════
# 3. A-Score Risk Heatmap
# ═══════════════════════════════════════════════
def draw_ascore():
    W, H = 1280, 800
    img = Image.new("RGBA", (W, H), BG)
    d = ImageDraw.Draw(img)

    font_title = get_font(26)
    font_sub = get_font(16)
    font_val = get_font(36)
    font_label = get_font(18)
    font_desc = get_font(14)

    # Header
    d.text((40, 30), "A-Score 风险热力图", fill=TEXT_PRIMARY, font=font_title)
    d.text((40, 70), "300054.SZ 鼎龙股份  |  评分日期: 2026-04-17", fill=TEXT_SECONDARY, font=font_sub)

    # Overall score circle (center top-right)
    cx, cy = 1080, 120
    score = 15.0
    score_pct = score / 100
    r = 70
    # Background ring
    d.ellipse([cx - r, cy - r, cx + r, cy + r], outline=BORDER, width=8)
    # Score arc (green = low risk)
    for angle in range(0, int(360 * score_pct)):
        rad = math.radians(angle - 90)
        x1 = cx + (r - 4) * math.cos(rad)
        y1 = cy + (r - 4) * math.sin(rad)
        x2 = cx + (r + 4) * math.cos(rad)
        y2 = cy + (r + 4) * math.sin(rad)
        d.line([(x1, y1), (x2, y2)], fill=GREEN, width=3)

    # Score text
    bbox = d.textbbox((0, 0), "15.0", font=font_val)
    tw = bbox[2] - bbox[0]
    d.text((cx - tw // 2, cy - 20), "15.0", fill=TEXT_PRIMARY, font=font_val)
    bbox = d.textbbox((0, 0), "低风险", font=font_label)
    tw = bbox[2] - bbox[0]
    d.text((cx - tw // 2, cy + 25), "低风险", fill=GREEN, font=font_label)

    # Legend
    legend_x, legend_y = 40, 120
    d.text((legend_x, legend_y), "风险等级：", fill=TEXT_SECONDARY, font=font_desc)
    for color, label, x_off in [(GREEN, "<40 安全", 80), (YELLOW, "40-70 监控", 180), (RED, ">70 高风险", 290)]:
        d.rounded_rectangle([legend_x + x_off, legend_y - 2, legend_x + x_off + 12, legend_y + 12], radius=2, fill=color)
        d.text((legend_x + x_off + 18, legend_y - 2), label, fill=TEXT_SECONDARY, font=font_desc)

    # Dimension cards
    dimensions = [
        ("财务造假层", 8.5, "M-Score / 现金流偏离 / 应收异常", GREEN),
        ("破产风险层", 3.2, "Altman Z-Score / 偿债能力", GREEN),
        ("非财务信号", 3.3, "股权质押 / 监管问询 / 减持", GREEN),
    ]

    card_w = 370
    card_h = 140
    gap_x = 30
    start_x = 40
    start_y = 180

    for idx, (name, val, desc, color) in enumerate(dimensions):
        col = idx % 3
        x = start_x + col * (card_w + gap_x)
        y = start_y

        # Card bg
        d.rounded_rectangle([x, y, x + card_w, y + card_h], radius=10, fill=CARD, outline=BORDER, width=1)

        # Left color bar
        d.rounded_rectangle([x, y + 15, x + 6, y + card_h - 15], radius=3, fill=color)

        # Dimension name
        d.text((x + 25, y + 15), name, fill=TEXT_PRIMARY, font=font_label)
        d.text((x + 25, y + 48), desc, fill=TEXT_SECONDARY, font=font_desc)

        # Score bar
        bar_y = y + 90
        bar_w = card_w - 50
        d.rounded_rectangle([x + 25, bar_y, x + 25 + bar_w, bar_y + 18], radius=9, fill="#21262d")
        fill_w = int(bar_w * min(val, 40) / 40)
        if fill_w > 0:
            d.rounded_rectangle([x + 25, bar_y, x + 25 + fill_w, bar_y + 18], radius=9, fill=color)

        # Score value
        score_text = f"{val:.1f}"
        bbox = d.textbbox((0, 0), score_text, font=font_val)
        tw = bbox[2] - bbox[0]
        d.text((x + card_w - 30 - tw, y + 15), score_text, fill=color, font=font_val)

    # Breakdown table
    table_y = 350
    d.text((40, table_y), "详细风险指标", fill=TEXT_PRIMARY, font=font_label)

    # Table header
    header_y = table_y + 40
    d.rectangle([40, header_y, 1240, header_y + 40], fill="#21262d", outline=BORDER, width=1)
    headers = ["指标", "数值", "风险等级", "说明"]
    header_xs = [60, 400, 580, 720]
    for hx, htext in zip(header_xs, headers):
        d.text((hx, header_y + 10), htext, fill=TEXT_PRIMARY, font=font_label)

    rows = [
        ("M-Score (Beneish)", "-1.82", "正常", "低于 -2.22 为操纵风险区"),
        ("现金流偏离度", "12.3%", "正常", "经营现金流 / 净利润 偏离不大"),
        ("应收账款占比", "8.5%", "正常", "远低于行业均值，回款健康"),
        ("毛利率波动", "±3.2%", "稳定", "连续三年波动在合理区间"),
        ("Altman Z-Score", "4.21", "安全", ">2.99 为安全区"),
        ("股权质押比例", "2.1%", "极低", "远低于警戒线 50%"),
    ]

    for ridx, (metric, value, level, note) in enumerate(rows):
        ry = header_y + 40 + ridx * 45
        bg = CARD if ridx % 2 == 0 else BG
        d.rectangle([40, ry, 1240, ry + 45], fill=bg, outline=BORDER, width=1)
        d.text((60, ry + 12), metric, fill=TEXT_PRIMARY, font=font_desc)
        d.text((400, ry + 12), value, fill=TEXT_PRIMARY, font=font_desc)
        d.text((580, ry + 12), level, fill=GREEN, font=font_desc)
        d.text((720, ry + 12), note, fill=TEXT_SECONDARY, font=font_desc)

    # Bottom insight banner (opaque dark-green bg for visibility on white themes)
    banner_y = 740
    d.rounded_rectangle([40, banner_y, 1240, banner_y + 40], radius=6, fill="#1a3a1e", outline=GREEN)
    d.text((60, banner_y + 10), "综合评估：该企业财务基本面稳健，未发现显著操纵或破产风险信号，属于低风险标的。", fill=GREEN, font=font_desc)

    img.save("docs/screenshots/ascore.png")
    print("[OK] Generated docs/screenshots/ascore.png")


if __name__ == "__main__":
    draw_logo()
    draw_kline()
    draw_ascore()
    print("\n[OK] All README assets generated!")
