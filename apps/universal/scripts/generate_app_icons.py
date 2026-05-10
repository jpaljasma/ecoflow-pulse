#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import json
import math

from PIL import Image, ImageChops, ImageDraw, ImageFilter


ROOT = Path(__file__).resolve().parents[1]
ASSETS = ROOT / "assets"
PUBLIC = ROOT / "public"
THEME_DEFINITIONS = ROOT / "theme-definitions.json"

MASTER_SVG_PATH = ASSETS / "pulsemark-v2-horizon-cut.svg"
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
NEW_DARK_SEMANTIC = THEME_DATA["themes"]["new-dark"]["semantic"]
BACKGROUND = NEW_DARK["background"]
CYAN = NEW_DARK["accentColor"]
TEAL = NEW_DARK_SEMANTIC["ac"]
GOLD = NEW_DARK_SEMANTIC["solar"]
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


def draw_smooth_line(
    image: Image.Image,
    points: list[tuple[float, float]],
    color: tuple[int, int, int, int],
    width: int,
    blur_radius: int = 0,
) -> None:
    layer = make_canvas(image.size[0])
    draw = ImageDraw.Draw(layer)
    draw.line(points, fill=color, width=width, joint="curve")
    if blur_radius:
        layer = layer.filter(ImageFilter.GaussianBlur(blur_radius))
    image.alpha_composite(layer)


def scale_points(points: list[tuple[float, float]], scale: int) -> list[tuple[float, float]]:
    return [(x * scale, y * scale) for x, y in points]


def draw_antialiased_polygon(
    image: Image.Image,
    points: list[tuple[float, float]],
    color: tuple[int, int, int, int],
    blur_radius: int = 0,
    scale: int = 4,
) -> None:
    if blur_radius:
        layer = make_canvas(image.size[0])
        ImageDraw.Draw(layer).polygon(points, fill=color)
        layer = layer.filter(ImageFilter.GaussianBlur(blur_radius))
        image.alpha_composite(layer)
        return

    layer = make_canvas(image.size[0] * scale)
    ImageDraw.Draw(layer).polygon(scale_points(points, scale), fill=color)
    layer = layer.resize(image.size, Image.Resampling.LANCZOS)
    image.alpha_composite(layer)


def draw_arc_stroke(
    image: Image.Image,
    bbox: tuple[float, float, float, float],
    start_degrees: float,
    end_degrees: float,
    color: tuple[int, int, int, int],
    width: int,
    blur_radius: int = 0,
) -> None:
    layer = make_canvas(image.size[0])
    draw = ImageDraw.Draw(layer)
    draw.arc(bbox, start=start_degrees, end=end_degrees, fill=color, width=width)
    if blur_radius:
        layer = layer.filter(ImageFilter.GaussianBlur(blur_radius))
    image.alpha_composite(layer)


def arc_point(
    center_x: float,
    center_y: float,
    radius_x: float,
    radius_y: float,
    degrees: float,
) -> tuple[float, float]:
    radians = math.radians(degrees)
    return (center_x + radius_x * math.cos(radians), center_y + radius_y * math.sin(radians))


def cubic_curve(
    start: tuple[float, float],
    control_one: tuple[float, float],
    control_two: tuple[float, float],
    end: tuple[float, float],
    steps: int,
) -> list[tuple[float, float]]:
    points: list[tuple[float, float]] = []
    for index in range(steps + 1):
        t = index / steps
        inverse = 1 - t
        points.append(
            (
                inverse**3 * start[0]
                + 3 * inverse * inverse * t * control_one[0]
                + 3 * inverse * t * t * control_two[0]
                + t**3 * end[0],
                inverse**3 * start[1]
                + 3 * inverse * inverse * t * control_one[1]
                + 3 * inverse * t * t * control_two[1]
                + t**3 * end[1],
            )
        )
    return points


def ellipse_points(
    center_x: float,
    center_y: float,
    radius_x: float,
    radius_y: float,
    start_degrees: float,
    end_degrees: float,
    steps: int,
) -> list[tuple[float, float]]:
    return [
        arc_point(
            center_x,
            center_y,
            radius_x,
            radius_y,
            start_degrees + (end_degrees - start_degrees) * index / steps,
        )
        for index in range(steps + 1)
    ]


def ellipse_through_points(
    start: tuple[float, float],
    end: tuple[float, float],
    start_degrees: float,
    end_degrees: float,
    steps: int,
) -> list[tuple[float, float]]:
    start_cos = math.cos(math.radians(start_degrees))
    end_cos = math.cos(math.radians(end_degrees))
    start_sin = math.sin(math.radians(start_degrees))
    end_sin = math.sin(math.radians(end_degrees))
    center_x = (start[0] * end_cos - end[0] * start_cos) / (end_cos - start_cos)
    radius_x = (start[0] - center_x) / start_cos
    center_y = (start[1] * end_sin - end[1] * start_sin) / (end_sin - start_sin)
    radius_y = (start[1] - center_y) / start_sin
    return ellipse_points(center_x, center_y, radius_x, radius_y, start_degrees, end_degrees, steps)


def horizon_cut_p_arc_points(
    size: int,
    bbox: tuple[float, float, float, float],
    start_degrees: float,
    width: int,
) -> list[tuple[float, float]]:
    left, top, right, bottom = bbox
    center_x = (left + right) / 2
    center_y = (top + bottom) / 2
    radius_x = (right - left) / 2
    radius_y = (bottom - top) / 2
    half_width = width / 2
    steps = 192
    outer_join_degrees = 344
    inner_join_degrees = 344
    outer = ellipse_points(
        center_x,
        center_y,
        radius_x + half_width,
        radius_y + half_width,
        start_degrees,
        outer_join_degrees,
        steps,
    )
    inner = ellipse_points(
        center_x,
        center_y,
        radius_x - half_width,
        radius_y - half_width,
        inner_join_degrees,
        start_degrees,
        steps,
    )
    top_outer_anchor = arc_point(
        center_x,
        center_y,
        radius_x + half_width,
        radius_y + half_width,
        outer_join_degrees,
    )
    top_inner_anchor = arc_point(
        center_x,
        center_y,
        radius_x - half_width,
        radius_y - half_width,
        inner_join_degrees,
    )
    tip = (size * 0.598, size * 0.656)
    outer_crescent = ellipse_through_points(top_outer_anchor, tip, -11.5, 73.0, 112)
    inner_crescent = ellipse_through_points(tip, top_inner_anchor, 71.0, -13.5, 112)
    return [*outer, *outer_crescent[1:], *inner_crescent[1:], *inner[1:]]


def svg_number(value: float) -> str:
    rounded = round(value, 1)
    if rounded == int(rounded):
        return str(int(rounded))
    return f"{rounded:.1f}"


def svg_polygon_path(points: list[tuple[float, float]]) -> str:
    first, *rest = points
    path = f"M{svg_number(first[0])} {svg_number(first[1])}"
    path += "".join(f"L{svg_number(x)} {svg_number(y)}" for x, y in rest)
    return f"{path}Z"


def draw_tapered_arc(
    image: Image.Image,
    bbox: tuple[float, float, float, float],
    start_degrees: float,
    end_degrees: float,
    color: tuple[int, int, int, int],
    width: int,
    blur_radius: int = 0,
) -> None:
    draw_antialiased_polygon(
        image,
        horizon_cut_p_arc_points(image.size[0], bbox, start_degrees, width),
        color,
        blur_radius,
    )


def horizon_y(size: int, x: float) -> float:
    center = size * 0.5
    span = size * 0.62
    normalized = max(-1.0, min(1.0, (x - center) / span))
    return size * 0.523 - size * 0.034 * (1 - normalized * normalized)


def horizon_points(size: int, steps: int = 144) -> list[tuple[float, float]]:
    return [(size * i / steps, horizon_y(size, size * i / steps)) for i in range(steps + 1)]


def mask_above_horizon(layer: Image.Image) -> Image.Image:
    size = layer.size[0]
    mask = Image.new("L", layer.size, 0)
    draw = ImageDraw.Draw(mask)
    top_polygon = [(0, 0), (size, 0), *reversed(horizon_points(size)), (0, horizon_y(size, 0))]
    draw.polygon(top_polygon, fill=255)
    alpha = layer.getchannel("A")
    layer.putalpha(ImageChops.multiply(alpha, mask))
    return layer


def draw_horizon_cut_motif(image: Image.Image, size: int, pulse_alpha: int = 255) -> None:
    draw = ImageDraw.Draw(image)
    arc_width = max(34, int(size * 0.061))
    arc_bbox = (size * 0.205, size * 0.18, size * 0.79, size * 0.765)

    draw_tapered_arc(image, arc_bbox, 180, 404, hex_rgba(CYAN, min(106, pulse_alpha)), arc_width + max(28, size // 22), int(size * 0.034))
    draw_tapered_arc(image, arc_bbox, 180, 404, hex_rgba("#083c58", min(150, pulse_alpha)), arc_width)
    draw_tapered_arc(image, arc_bbox, 180, 404, hex_rgba(CYAN, min(255, pulse_alpha)), arc_width)
    draw_arc_stroke(
        image,
        (size * 0.228, size * 0.205, size * 0.765, size * 0.73),
        195,
        318,
        hex_rgba("#bbf5ff", min(92, pulse_alpha)),
        max(4, size // 150),
    )

    stem = [
        (size * 0.17, size * 0.565),
        (size * 0.225, size * 0.565),
        (size * 0.225, size * 0.742),
        (size * 0.17, size * 0.797),
    ]
    stem_glow = make_canvas(size)
    ImageDraw.Draw(stem_glow).polygon(stem, fill=hex_rgba(CYAN, min(132, pulse_alpha)))
    image.alpha_composite(stem_glow.filter(ImageFilter.GaussianBlur(int(size * 0.032))))
    draw_antialiased_polygon(image, stem, hex_rgba(CYAN, min(246, pulse_alpha)))
    draw.line(
        [(size * 0.216, size * 0.58), (size * 0.216, size * 0.732), (size * 0.184, size * 0.764)],
        fill=hex_rgba("#9eefff", min(100, pulse_alpha)),
        width=max(3, size // 210),
    )
    draw.line(
        [(size * 0.169, size * 0.574), (size * 0.169, size * 0.786)],
        fill=hex_rgba("#062238", min(160, pulse_alpha)),
        width=max(3, size // 190),
    )

    sun = make_canvas(size)
    sun_draw = ImageDraw.Draw(sun)
    sun_box = (size * 0.424, size * 0.425, size * 0.576, size * 0.577)
    sun_draw.ellipse(sun_box, fill=hex_rgba(GOLD, min(246, pulse_alpha)))
    sun_draw.ellipse(
        (size * 0.442, size * 0.442, size * 0.558, size * 0.558),
        fill=hex_rgba("#ffe48a", min(236, pulse_alpha)),
    )
    image.alpha_composite(mask_above_horizon(sun.filter(ImageFilter.GaussianBlur(max(1, size // 240)))))

    sun_glow = make_canvas(size)
    add_glow(
        sun_glow,
        (int(size * 0.345), int(size * 0.355), int(size * 0.655), int(size * 0.635)),
        hex_rgba(GOLD, min(84, pulse_alpha)),
        int(size * 0.042),
    )
    image.alpha_composite(mask_above_horizon(sun_glow))

    draw_smooth_line(image, horizon_points(size), hex_rgba(GOLD, min(92, pulse_alpha)), max(6, size // 92), int(size * 0.014))
    draw_smooth_line(image, horizon_points(size), hex_rgba(GOLD, min(236, pulse_alpha)), max(2, size // 300))
    draw_smooth_line(image, horizon_points(size), hex_rgba("#fff1b2", min(116, pulse_alpha)), max(1, size // 520))


def build_icon(size: int = 1024) -> Image.Image:
    image = make_canvas(size, hex_rgba(BACKGROUND))
    px = image.load()
    for y in range(size):
        for x in range(size):
            mix = (x + y) / (2 * (size - 1))
            px[x, y] = blend(BACKGROUND, TEAL, mix * 0.25)

    add_glow(
        image,
        (int(size * 0.06), int(size * 0.02), int(size * 0.78), int(size * 0.54)),
        hex_rgba(CYAN, 42),
        int(size * 0.13),
    )
    add_glow(
        image,
        (int(size * 0.25), int(size * 0.28), int(size * 0.75), int(size * 0.66)),
        hex_rgba(GOLD, 46),
        int(size * 0.105),
    )

    inner = make_canvas(size)
    inner_draw = ImageDraw.Draw(inner)
    inset = int(size * 0.06)
    inner_draw.rounded_rectangle(
        (inset, inset, size - inset, size - inset),
        radius=int(size * 0.24),
        fill=hex_rgba("#132033", 30),
        outline=hex_rgba(WHITE, 20),
        width=max(2, size // 256),
    )
    inner = inner.filter(ImageFilter.GaussianBlur(int(size * 0.01)))
    image.alpha_composite(inner)

    draw_horizon_cut_motif(image, size)

    return apply_rounded_mask(image, int(size * 0.225))


def build_adaptive_foreground(size: int = 1024) -> Image.Image:
    image = make_canvas(size)
    halo = make_canvas(size)
    add_glow(
        halo,
        (int(size * 0.2), int(size * 0.2), int(size * 0.8), int(size * 0.8)),
        hex_rgba(CYAN, 74),
        int(size * 0.08),
    )
    image.alpha_composite(halo)
    draw_horizon_cut_motif(image, size, pulse_alpha=255)
    return image


def build_master_svg() -> str:
    size = 1024
    arc_width = max(34, int(size * 0.061))
    arc_path = svg_polygon_path(
        horizon_cut_p_arc_points(
            size,
            (size * 0.205, size * 0.18, size * 0.79, size * 0.765),
            180,
            arc_width,
        )
    )
    stem_path = svg_polygon_path(
        [
            (size * 0.17, size * 0.565),
            (size * 0.225, size * 0.565),
            (size * 0.225, size * 0.742),
            (size * 0.17, size * 0.797),
        ]
    )
    return f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" role="img" aria-label="PulseMark v2 Horizon Cut P">
  <defs>
    <linearGradient id="tile" x1="120" y1="80" x2="910" y2="960" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="{BACKGROUND}"/>
      <stop offset="0.56" stop-color="#102033"/>
      <stop offset="1" stop-color="#17303f"/>
    </linearGradient>
    <linearGradient id="cyanStroke" x1="240" y1="230" x2="730" y2="720" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="#8ff1ff"/>
      <stop offset="0.48" stop-color="{CYAN}"/>
      <stop offset="1" stop-color="#137aa8"/>
    </linearGradient>
    <linearGradient id="sun" x1="512" y1="402" x2="512" y2="548" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="#fff1a8"/>
      <stop offset="1" stop-color="{GOLD}"/>
    </linearGradient>
    <filter id="cyanGlow" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="22" result="blur"/>
      <feColorMatrix in="blur" type="matrix" values="0 0 0 0 0.32 0 0 0 0 0.84 0 0 0 0 1 0 0 0 0.64 0"/>
      <feMerge><feMergeNode/><feMergeNode in="SourceGraphic"/></feMerge>
    </filter>
    <filter id="goldGlow" x="-30%" y="-30%" width="160%" height="160%">
      <feGaussianBlur stdDeviation="26" result="blur"/>
      <feColorMatrix in="blur" type="matrix" values="0 0 0 0 1 0 0 0 0 0.76 0 0 0 0 0.35 0 0 0 0.62 0"/>
      <feMerge><feMergeNode/><feMergeNode in="SourceGraphic"/></feMerge>
    </filter>
    <clipPath id="aboveHorizon">
      <path d="M0 0H1024V536C792 499 620 492 512 500C356 511 210 526 0 546Z"/>
    </clipPath>
  </defs>
  <rect width="1024" height="1024" rx="230" fill="url(#tile)"/>
  <rect x="61" y="61" width="902" height="902" rx="246" fill="#132033" fill-opacity="0.12" stroke="#f4f7fb" stroke-opacity="0.08" stroke-width="5"/>
  <path d="{arc_path}" fill="url(#cyanStroke)" filter="url(#cyanGlow)"/>
  <path d="{stem_path}" fill="url(#cyanStroke)" filter="url(#cyanGlow)"/>
  <circle cx="512" cy="499" r="76" fill="url(#sun)" clip-path="url(#aboveHorizon)" filter="url(#goldGlow)"/>
  <path d="M0 546C238 512 402 501 512 500C686 498 842 511 1024 536" fill="none" stroke="{GOLD}" stroke-width="4" stroke-linecap="round" filter="url(#goldGlow)"/>
</svg>
"""


def save_resized(image: Image.Image, path: Path, size: int) -> None:
    image.resize((size, size), Image.Resampling.LANCZOS).save(path)


def text_width(draw: ImageDraw.ImageDraw, text: str, font_size: int) -> int:
    bbox = draw.textbbox((0, 0), text, font_size=font_size)
    return bbox[2] - bbox[0]


def wrap_text(draw: ImageDraw.ImageDraw, text: str, max_width: int, font_size: int) -> list[str]:
    words = text.split()
    lines: list[str] = []
    current = ""
    for word in words:
        candidate = f"{current} {word}".strip()
        if current and text_width(draw, candidate, font_size) > max_width:
            lines.append(current)
            current = word
        else:
            current = candidate
    if current:
        lines.append(current)
    return lines


def draw_wrapped_text(
    draw: ImageDraw.ImageDraw,
    xy: tuple[int, int],
    text: str,
    max_width: int,
    fill: tuple[int, int, int, int],
    font_size: int,
    line_gap: int,
) -> int:
    x, y = xy
    for line in wrap_text(draw, text, max_width, font_size):
        draw.text((x, y), line, fill=fill, font_size=font_size)
        bbox = draw.textbbox((x, y), line, font_size=font_size)
        y = bbox[3] + line_gap
    return y


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
            fill=hex_rgba(CYAN, 18),
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
        fill=hex_rgba(CYAN, 24),
        outline=hex_rgba(CYAN, 86),
    )
    shard_draw.polygon(
        [(830, 0), (1200, 0), (1200, 210), (985, 255)],
        fill=hex_rgba(GOLD, 26),
        outline=hex_rgba(GOLD, 94),
    )
    shard_draw.polygon(
        [(865, 630), (1200, 445), (1200, 630)],
        fill=hex_rgba(CYAN, 22),
        outline=hex_rgba(CYAN, 78),
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
    rail_draw.line([(0, 500), (470, 335), (1200, 385)], fill=hex_rgba(CYAN, 136), width=4)
    rail_draw.line([(0, 548), (505, 372), (1200, 422)], fill=hex_rgba(WHITE, 64), width=2)
    rail_draw.line([(750, 0), (540, 286), (780, 630)], fill=hex_rgba(GOLD, 120), width=3)
    rail_draw.line([(804, 0), (594, 286), (834, 630)], fill=hex_rgba(WHITE, 48), width=1)
    rails = rails.filter(ImageFilter.GaussianBlur(1.2))
    image.alpha_composite(rails)

    add_glow(image, (-60, 38, 520, 470), hex_rgba(CYAN, 92), 120)
    add_glow(image, (770, 90, 1280, 660), hex_rgba(GOLD, 72), 132)
    add_glow(image, (490, 210, 1010, 590), hex_rgba("#7df4d4", 38), 150)


def main() -> None:
    ASSETS.mkdir(parents=True, exist_ok=True)
    PUBLIC.mkdir(parents=True, exist_ok=True)

    MASTER_SVG_PATH.write_text(build_master_svg(), encoding="utf-8")

    icon = build_icon(1024)
    icon.save(ICON_PATH)
    icon.save(APPLE_TOUCH_PATH)
    icon.save(PUBLIC_APPLE_TOUCH_PATH)

    adaptive = build_adaptive_foreground(1024)
    adaptive.save(ADAPTIVE_FOREGROUND_PATH)

    save_resized(icon, FAVICON_PATH, 64)
    build_share_card().save(PUBLIC_SHARE_PATH)
    PUBLIC_INDEX_PATH.write_text(build_web_index_template(), encoding="utf-8")

    print(f"wrote {MASTER_SVG_PATH.relative_to(ROOT)}")
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
    subject = "PULSE ENERGY INTELLIGENCE"

    draw.rounded_rectangle((594, 126, 1108, 176), radius=25, fill=hex_rgba("#081625", 188), outline=hex_rgba(CYAN, 108), width=2)
    draw.line((594, 196, 1118, 196), fill=hex_rgba(WHITE, 42), width=2)
    draw.line((594, 200, 1080, 200), fill=hex_rgba(CYAN, 78), width=4)
    draw.text((624, 138), subject, fill=hex_rgba(CYAN), font_size=28)
    next_y = draw_wrapped_text(draw, (594, 218), title, 560, hex_rgba(WHITE), 58, 10)
    next_y = draw_wrapped_text(draw, (594, next_y + 18), description, 560, hex_rgba(WHITE, 214), 30, 8)
    draw.text((594, max(510, next_y + 16)), "Premium live solar + battery intelligence", fill=hex_rgba(GOLD, 224), font_size=28)
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
