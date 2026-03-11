#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import json

from PIL import Image, ImageChops, ImageDraw, ImageFilter


ROOT = Path(__file__).resolve().parents[1]
ASSETS = ROOT / "assets"
PUBLIC = ROOT / "public"
THEME_DEFINITIONS = ROOT / "theme-definitions.json"

ICON_PATH = ASSETS / "icon.png"
ADAPTIVE_FOREGROUND_PATH = ASSETS / "adaptive-icon-foreground.png"
FAVICON_PATH = ASSETS / "favicon.png"
APPLE_TOUCH_PATH = ASSETS / "apple-touch-icon.png"
PUBLIC_APPLE_TOUCH_PATH = PUBLIC / "apple-touch-icon.png"
PUBLIC_SHARE_PATH = PUBLIC / "social-share.png"
PUBLIC_INDEX_PATH = PUBLIC / "index.html"

with THEME_DEFINITIONS.open() as f:
    THEME_DATA = json.load(f)

NEW_DARK = THEME_DATA["themes"]["new-dark"]["colors"]
BACKGROUND = NEW_DARK["background"]
MINT = NEW_DARK["accentColor"]
TEAL = NEW_DARK["backgroundFocus"]
LIME = "#c8ff5a"
WHITE = NEW_DARK["color"]


def make_canvas(size: int, color: str | tuple[int, int, int, int] = (0, 0, 0, 0)) -> Image.Image:
    return Image.new("RGBA", (size, size), color)


def hex_rgba(value: str, alpha: int = 255) -> tuple[int, int, int, int]:
    value = value.lstrip("#")
    return tuple(int(value[i : i + 2], 16) for i in (0, 2, 4)) + (alpha,)


def blend(a: str, b: str, t: float, alpha: int = 255) -> tuple[int, int, int, int]:
    ar, ag, ab, _ = hex_rgba(a)
    br, bg, bb, _ = hex_rgba(b)
    return (
        int(ar + (br - ar) * t),
        int(ag + (bg - ag) * t),
        int(ab + (bb - ab) * t),
        alpha,
    )


def apply_rounded_mask(image: Image.Image, radius: int) -> Image.Image:
    mask = Image.new("L", image.size, 0)
    ImageDraw.Draw(mask).rounded_rectangle((0, 0, image.size[0], image.size[1]), radius=radius, fill=255)
    image.putalpha(mask)
    return image


def add_glow(
    image: Image.Image,
    bbox: tuple[int, int, int, int],
    color: tuple[int, int, int, int],
    blur_radius: int,
) -> None:
    layer = make_canvas(image.size[0])
    draw = ImageDraw.Draw(layer)
    draw.ellipse(bbox, fill=color)
    layer = layer.filter(ImageFilter.GaussianBlur(blur_radius))
    image.alpha_composite(layer)


def draw_panel_grid(image: Image.Image, size: int) -> None:
    draw = ImageDraw.Draw(image)
    left = int(size * 0.17)
    top = int(size * 0.68)
    right = int(size * 0.83)
    bottom = int(size * 0.84)
    draw.rounded_rectangle(
        (left, top, right, bottom),
        radius=int(size * 0.045),
        outline=hex_rgba(WHITE, 54),
        width=max(2, size // 128),
    )
    columns = 4
    rows = 2
    for i in range(1, columns):
        x = left + int((right - left) * i / columns)
        draw.line((x, top + 14, x, bottom - 14), fill=hex_rgba(WHITE, 38), width=max(2, size // 192))
    for i in range(1, rows):
        y = top + int((bottom - top) * i / rows)
        draw.line((left + 14, y, right - 14, y), fill=hex_rgba(WHITE, 38), width=max(2, size // 192))


def draw_motif(image: Image.Image, size: int, pulse_alpha: int = 255) -> None:
    draw = ImageDraw.Draw(image)
    stroke = max(18, size // 42)

    ring_bounds = (
        int(size * 0.17),
        int(size * 0.17),
        int(size * 0.83),
        int(size * 0.83),
    )
    draw.arc(ring_bounds, start=210, end=28, fill=hex_rgba(MINT, 228), width=stroke)
    draw.arc(
        (
            int(size * 0.23),
            int(size * 0.23),
            int(size * 0.77),
            int(size * 0.77),
        ),
        start=24,
        end=142,
        fill=hex_rgba(LIME, 176),
        width=max(10, stroke // 2),
    )

    pulse_points = [
        (int(size * 0.22), int(size * 0.58)),
        (int(size * 0.39), int(size * 0.58)),
        (int(size * 0.46), int(size * 0.46)),
        (int(size * 0.53), int(size * 0.66)),
        (int(size * 0.61), int(size * 0.53)),
        (int(size * 0.78), int(size * 0.53)),
    ]
    draw.line(
        pulse_points,
        fill=hex_rgba(WHITE, pulse_alpha),
        width=max(18, size // 34),
        joint="curve",
    )
    node_r = max(10, size // 48)
    for point, color in ((pulse_points[0], MINT), (pulse_points[-1], LIME)):
        x, y = point
        draw.ellipse((x - node_r, y - node_r, x + node_r, y + node_r), fill=hex_rgba(color, pulse_alpha))

    bolt = [
        (int(size * 0.56), int(size * 0.24)),
        (int(size * 0.42), int(size * 0.50)),
        (int(size * 0.53), int(size * 0.50)),
        (int(size * 0.45), int(size * 0.78)),
        (int(size * 0.64), int(size * 0.44)),
        (int(size * 0.53), int(size * 0.44)),
    ]
    draw.polygon(bolt, fill=hex_rgba(WHITE, 255))

    spark = [
        (int(size * 0.72), int(size * 0.24)),
        (int(size * 0.75), int(size * 0.20)),
        (int(size * 0.78), int(size * 0.24)),
        (int(size * 0.82), int(size * 0.27)),
        (int(size * 0.78), int(size * 0.30)),
        (int(size * 0.75), int(size * 0.34)),
        (int(size * 0.72), int(size * 0.30)),
        (int(size * 0.68), int(size * 0.27)),
    ]
    draw.polygon(spark, fill=hex_rgba(LIME, 220))


def build_icon(size: int = 1024) -> Image.Image:
    image = make_canvas(size, hex_rgba(BACKGROUND))
    px = image.load()
    for y in range(size):
        for x in range(size):
            mix = (x + y) / (2 * (size - 1))
            px[x, y] = blend(BACKGROUND, TEAL, mix * 0.92)

    add_glow(
        image,
        (int(size * 0.02), int(size * 0.02), int(size * 0.65), int(size * 0.58)),
        hex_rgba(MINT, 115),
        int(size * 0.11),
    )
    add_glow(
        image,
        (int(size * 0.42), int(size * 0.44), int(size * 0.98), int(size * 1.02)),
        hex_rgba(LIME, 88),
        int(size * 0.15),
    )

    inner = make_canvas(size)
    inner_draw = ImageDraw.Draw(inner)
    inset = int(size * 0.06)
    inner_draw.rounded_rectangle(
        (inset, inset, size - inset, size - inset),
        radius=int(size * 0.24),
        fill=hex_rgba("#0d3934", 84),
        outline=hex_rgba(WHITE, 18),
        width=max(2, size // 256),
    )
    inner = inner.filter(ImageFilter.GaussianBlur(int(size * 0.01)))
    image.alpha_composite(inner)

    orbit = make_canvas(size)
    orbit_draw = ImageDraw.Draw(orbit)
    orbit_draw.arc(
        (int(size * 0.08), int(size * 0.11), int(size * 0.92), int(size * 0.94)),
        start=192,
        end=345,
        fill=hex_rgba(WHITE, 34),
        width=max(5, size // 170),
    )
    orbit_draw.arc(
        (int(size * 0.14), int(size * 0.08), int(size * 0.88), int(size * 0.76)),
        start=16,
        end=140,
        fill=hex_rgba(MINT, 56),
        width=max(4, size // 220),
    )
    image.alpha_composite(orbit)

    draw_panel_grid(image, size)
    draw_motif(image, size)

    return apply_rounded_mask(image, int(size * 0.225))


def build_adaptive_foreground(size: int = 1024) -> Image.Image:
    image = make_canvas(size)
    halo = make_canvas(size)
    add_glow(
        halo,
        (int(size * 0.2), int(size * 0.2), int(size * 0.8), int(size * 0.8)),
        hex_rgba(MINT, 74),
        int(size * 0.08),
    )
    image.alpha_composite(halo)
    draw_motif(image, size, pulse_alpha=255)
    return image


def save_resized(image: Image.Image, path: Path, size: int) -> None:
    image.resize((size, size), Image.Resampling.LANCZOS).save(path)


def draw_share_background(image: Image.Image) -> None:
    width, height = image.size
    px = image.load()
    for y in range(height):
        for x in range(width):
            x_mix = x / max(1, width - 1)
            y_mix = y / max(1, height - 1)
            mix = min(1.0, 0.12 + x_mix * 0.58 + y_mix * 0.3)
            px[x, y] = blend(BACKGROUND, TEAL, mix)

    haze = Image.new("RGBA", image.size, (0, 0, 0, 0))
    haze_draw = ImageDraw.Draw(haze)
    haze_draw.polygon(
        [(0, 0), (width * 0.64, 0), (width * 0.4, height), (0, height)],
        fill=hex_rgba("#051916", 118),
    )
    haze_draw.polygon(
        [(width * 0.54, 0), (width, 0), (width, height * 0.72), (width * 0.72, height)],
        fill=hex_rgba("#122f2a", 92),
    )
    haze = haze.filter(ImageFilter.GaussianBlur(18))
    image.alpha_composite(haze)

    grid = Image.new("RGBA", image.size, (0, 0, 0, 0))
    grid_draw = ImageDraw.Draw(grid)
    for offset in range(-height, width, 96):
        grid_draw.line(
            [(offset, 0), (offset + height, height)],
            fill=hex_rgba(WHITE, 16),
            width=2,
        )
    for offset in range(-height // 2, width, 160):
        grid_draw.line(
            [(offset, 0), (offset + int(height * 1.15), height)],
            fill=hex_rgba(MINT, 18),
            width=4,
        )
    for y in range(80, height, 92):
        grid_draw.line([(0, y), (width, y)], fill=hex_rgba(WHITE, 10), width=2)
    grid = grid.filter(ImageFilter.GaussianBlur(0.4))
    image.alpha_composite(grid)

    shards = Image.new("RGBA", image.size, (0, 0, 0, 0))
    shard_draw = ImageDraw.Draw(shards)
    shard_draw.polygon(
        [(0, 0), (425, 0), (270, 230), (0, 320)],
        fill=hex_rgba(MINT, 24),
        outline=hex_rgba(MINT, 86),
    )
    shard_draw.polygon(
        [(830, 0), (1200, 0), (1200, 210), (985, 255)],
        fill=hex_rgba(LIME, 26),
        outline=hex_rgba(LIME, 94),
    )
    shard_draw.polygon(
        [(865, 630), (1200, 445), (1200, 630)],
        fill=hex_rgba(MINT, 22),
        outline=hex_rgba(MINT, 78),
    )
    shard_draw.polygon(
        [(520, 630), (760, 405), (1010, 630)],
        fill=hex_rgba(WHITE, 10),
        outline=hex_rgba(WHITE, 34),
    )
    shards = shards.filter(ImageFilter.GaussianBlur(1))
    image.alpha_composite(shards)

    rails = Image.new("RGBA", image.size, (0, 0, 0, 0))
    rail_draw = ImageDraw.Draw(rails)
    rail_draw.line([(0, 500), (470, 335), (1200, 385)], fill=hex_rgba(MINT, 136), width=4)
    rail_draw.line([(0, 548), (505, 372), (1200, 422)], fill=hex_rgba(WHITE, 64), width=2)
    rail_draw.line([(750, 0), (540, 286), (780, 630)], fill=hex_rgba(LIME, 120), width=3)
    rail_draw.line([(804, 0), (594, 286), (834, 630)], fill=hex_rgba(WHITE, 48), width=1)
    rails = rails.filter(ImageFilter.GaussianBlur(1.2))
    image.alpha_composite(rails)

    add_glow(image, (-60, 38, 520, 470), hex_rgba(MINT, 92), 120)
    add_glow(image, (770, 90, 1280, 660), hex_rgba(LIME, 72), 132)
    add_glow(image, (490, 210, 1010, 590), hex_rgba("#7df4d4", 38), 150)


def main() -> None:
    ASSETS.mkdir(parents=True, exist_ok=True)
    PUBLIC.mkdir(parents=True, exist_ok=True)

    icon = build_icon(1024)
    icon.save(ICON_PATH)
    icon.save(APPLE_TOUCH_PATH)
    icon.save(PUBLIC_APPLE_TOUCH_PATH)

    adaptive = build_adaptive_foreground(1024)
    adaptive.save(ADAPTIVE_FOREGROUND_PATH)

    save_resized(icon, FAVICON_PATH, 64)
    build_share_card().save(PUBLIC_SHARE_PATH)
    PUBLIC_INDEX_PATH.write_text(build_web_index_template(), encoding="utf-8")

    print(f"wrote {ICON_PATH.relative_to(ROOT)}")
    print(f"wrote {ADAPTIVE_FOREGROUND_PATH.relative_to(ROOT)}")
    print(f"wrote {FAVICON_PATH.relative_to(ROOT)}")
    print(f"wrote {APPLE_TOUCH_PATH.relative_to(ROOT)}")
    print(f"wrote {PUBLIC_APPLE_TOUCH_PATH.relative_to(ROOT)}")
    print(f"wrote {PUBLIC_SHARE_PATH.relative_to(ROOT)}")
    print(f"wrote {PUBLIC_INDEX_PATH.relative_to(ROOT)}")


def build_share_card() -> Image.Image:
    width, height = 1200, 630
    image = Image.new("RGBA", (width, height), hex_rgba(BACKGROUND))
    draw_share_background(image)

    icon = build_icon(420)
    image.alpha_composite(icon, (88, 104))

    draw = ImageDraw.Draw(image)
    title = THEME_DATA["metadata"]["shareTitle"]
    description = THEME_DATA["metadata"]["shareDescription"]
    subject = THEME_DATA["metadata"]["subject"].upper()

    draw.rounded_rectangle((594, 126, 1030, 176), radius=25, fill=hex_rgba("#03221e", 182), outline=hex_rgba(MINT, 108), width=2)
    draw.line((594, 196, 1118, 196), fill=hex_rgba(WHITE, 42), width=2)
    draw.line((594, 200, 1080, 200), fill=hex_rgba(MINT, 78), width=4)
    draw.text((624, 138), subject, fill=hex_rgba(MINT), font_size=28)
    draw.text((594, 220), title, fill=hex_rgba(WHITE), font_size=70)
    draw.text((594, 360), description, fill=hex_rgba(WHITE, 214), font_size=34)
    draw.text((594, 500), "Live solar + battery + backup power intelligence", fill=hex_rgba(LIME, 224), font_size=30)
    return image


def build_web_index_template() -> str:
    meta = THEME_DATA["metadata"]
    return f"""<!DOCTYPE html>
<html lang="%LANG_ISO_CODE%">
  <head>
    <meta charset="utf-8" />
    <meta httpEquiv="X-UA-Compatible" content="IE=edge" />
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no" />
    <title>%WEB_TITLE%</title>
    <meta name="subject" content="{meta["subject"]}" />
    <meta property="og:type" content="website" />
    <meta property="og:title" content="{meta["shareTitle"]}" />
    <meta property="og:description" content="{meta["shareDescription"]}" />
    <meta property="og:image" content="/social-share.png" />
    <meta property="og:site_name" content="{meta["title"]}" />
    <meta name="twitter:card" content="summary_large_image" />
    <meta name="twitter:title" content="{meta["shareTitle"]}" />
    <meta name="twitter:description" content="{meta["shareDescription"]}" />
    <meta name="twitter:image" content="/social-share.png" />
    <link rel="apple-touch-icon" href="/apple-touch-icon.png" />
    <style id="expo-reset">
      html,
      body {{
        height: 100%;
      }}
      body {{
        overflow: hidden;
      }}
      #root {{
        display: flex;
        height: 100%;
        flex: 1;
      }}
    </style>
  </head>
  <body>
    <noscript>
      You need to enable JavaScript to run this app.
    </noscript>
    <div id="root"></div>
  </body>
</html>
"""


if __name__ == "__main__":
    main()
