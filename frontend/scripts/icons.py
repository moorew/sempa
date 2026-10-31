#!/usr/bin/env python3
"""
Generate every Sempa app icon from one definition (1.29 icon).

The icon is the "day thread": a column of nodes on a fine line, the first one
checked off, beside three task bars, on a terracotta tile. All geometry lives
in this file on a 100-unit tile grid; the SVGs are written from it and every
PNG is rasterised straight from those SVGs at its final size (no downscaling
of one big bitmap), so small sizes stay crisp.

The check is knocked out of its circle and the thread is translucent cream, so
the same glyph works over any background colour. That matters for Android's
theme-matched launcher icons, which put one foreground over six theme colours.

Writes (paths relative to the repo root):
  frontend/src-tauri/icons/*            Tauri desktop (Windows + Linux), Windows tiles, .ico, .icns
  sempa-linux/assets/icon.*             Linux/desktop raster + SVG source
  sempa-linux/desktop/icons/hicolor/**  freedesktop icon theme (PNG sizes, scalable, symbolic)
  frontend/static/**                    web favicons, PWA icons (any + maskable), apple-touch-icon
  frontend/android/app/src/main/res/mipmap-*/ic_launcher{,_round,_foreground}.png
  frontend/resources/ios/AppIcon-1024.png   copied into the generated iOS project by CI
  ../sempa-web/{favicon.ico,assets/*}   the website, when that repo is checked out next to this one

Requires Python 3 with Pillow and Playwright (Chromium) for rasterising, plus
ImageMagick `convert` for the multi-size .ico files.

Run from anywhere:  python3 frontend/scripts/icons.py
"""
import io
import os
import subprocess
import tempfile

from PIL import Image
from playwright.sync_api import sync_playwright

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..'))
WEB = os.path.abspath(os.path.join(ROOT, '..', 'sempa-web'))

# ── Palette ────────────────────────────────────────────────────────────────
GROUND = '#b3592e'      # terracotta, the brand accent
CREAM = '#fdf6ee'
HONEY = '#f4c56e'       # the one warm pop: today's task in progress
SYMBOLIC = '#2e3436'    # freedesktop symbolic icons are recoloured by the theme

# ── Glyph geometry (100-unit tile; centred on 50,50) ───────────────────────
ROWS = (32.5, 52.5, 72.5)
NODE_X = 27.0
CHECK_R = 8.75
DOT_R = 4.625
BAR_X = 40.5
BAR_H = 7.75
BAR_LENS = (41.0, 35.0, 26.0)
THREAD_W = 2.25
TILE_RX = 22.5


def glyph(scale=1.0, thread=True, mono=None, uid='g', small=False):
    """The mark alone, as SVG elements on the 100-unit grid, scaled about 50,50.

    mono: draw everything in one solid colour (Android monochrome-style use).
    small: 32px and under, where the softened last row would wash out; draw it solid.
    """
    c1 = mono or CREAM
    c2 = mono or HONEY
    soft = '' if (mono or small) else ' fill-opacity="0.72"'
    y0, y1, y2 = ROWS
    parts = []
    if thread:
        # Two segments that stop at the shapes, so nothing shows through the
        # knocked-out check or the translucent last dot.
        op = '' if mono else ' fill-opacity="0.38"'
        for top, bot in ((y0 + CHECK_R, y1 - DOT_R), (y1 + DOT_R, y2 - DOT_R)):
            parts.append(f'<rect x="{NODE_X - THREAD_W / 2}" y="{top}" width="{THREAD_W}" height="{bot - top}" fill="{c1}"{op}/>')
    cx, cy = NODE_X, y0
    parts.append(
        f'<mask id="{uid}-chk" maskUnits="userSpaceOnUse" x="0" y="0" width="100" height="100">'
        f'<circle cx="{cx}" cy="{cy}" r="{CHECK_R}" fill="#fff"/>'
        f'<path d="M{cx - 3.875},{cy + 0.25} l2.75,2.75 l5.125,-5.5" fill="none" stroke="#000" '
        f'stroke-width="2.875" stroke-linecap="round" stroke-linejoin="round"/></mask>'
        f'<circle cx="{cx}" cy="{cy}" r="{CHECK_R}" fill="{c1}" mask="url(#{uid}-chk)"/>'
    )
    parts.append(f'<circle cx="{NODE_X}" cy="{y1}" r="{DOT_R}" fill="{c2}"/>')
    parts.append(f'<circle cx="{NODE_X}" cy="{y2}" r="{DOT_R}" fill="{c1}"{soft}/>')
    fills = ((c1, ''), (c2, ''), (c1, soft))
    for (y, ln, (col, extra)) in zip(ROWS, BAR_LENS, fills):
        parts.append(f'<rect x="{BAR_X}" y="{y - BAR_H / 2}" width="{ln}" height="{BAR_H}" rx="{BAR_H / 2}" fill="{col}"{extra}/>')
    body = ''.join(parts)
    if scale != 1.0:
        body = f'<g transform="translate(50 50) scale({scale}) translate(-50 -50)">{body}</g>'
    return body


def svg(content, view='0 0 100 100', size=None):
    wh = f' width="{size}" height="{size}"' if size else ''
    return f'<svg xmlns="http://www.w3.org/2000/svg"{wh} viewBox="{view}">{content}</svg>'


def tile_svg(shape='rounded', scale=1.0, thread=True, size=None, small=False):
    """shape: rounded (transparent corners), square (opaque, platform masks it), circle."""
    if shape == 'rounded':
        ground = f'<rect width="100" height="100" rx="{TILE_RX}" fill="{GROUND}"/>'
    elif shape == 'circle':
        ground = f'<circle cx="50" cy="50" r="50" fill="{GROUND}"/>'
    else:
        ground = f'<rect width="100" height="100" fill="{GROUND}"/>'
    return svg(ground + glyph(scale, thread, small=small), size=size)


def adaptive_foreground_svg(size=None):
    """Android adaptive foreground: 108dp layer, of which the middle 72dp shows.
    The 100-unit tile maps onto that 72dp, so the glyph keeps the same
    proportions as every other platform (and sits well inside the 66dp safe circle)."""
    inner = f'<g transform="translate(18 18) scale(0.72)">{glyph(uid="fg")}</g>'
    return svg(inner, view='0 0 108 108', size=size)


def symbolic_svg():
    """16px monochrome for GNOME/KDE trays and notifications (theme recolours it)."""
    c = SYMBOLIC
    return (
        '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16">\n'
        '  <!-- Sempa symbolic icon: the day thread (checked first node + task bars),\n'
        '       monochrome; recoloured by the desktop theme. Generated by frontend/scripts/icons.py. -->\n'
        '  <mask id="m" maskUnits="userSpaceOnUse" x="0" y="0" width="16" height="16">\n'
        '    <circle cx="3.25" cy="3.75" r="2.75" fill="#fff"/>\n'
        '    <path d="M2,3.85 l0.85,0.85 l1.6,-1.7" fill="none" stroke="#000" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>\n'
        '  </mask>\n'
        f'  <circle cx="3.25" cy="3.75" r="2.75" fill="{c}" mask="url(#m)"/>\n'
        f'  <circle cx="3.25" cy="8.5" r="1.5" fill="{c}"/>\n'
        f'  <circle cx="3.25" cy="13" r="1.5" fill="{c}"/>\n'
        f'  <rect x="7.5" y="2.75" width="8" height="2" rx="1" fill="{c}"/>\n'
        f'  <rect x="7.5" y="7.5" width="6.5" height="2" rx="1" fill="{c}"/>\n'
        f'  <rect x="7.5" y="12" width="5" height="2" rx="1" fill="{c}"/>\n'
        '</svg>\n'
    )


# ── Rasterising ────────────────────────────────────────────────────────────
class Raster:
    def __init__(self):
        self.pw = sync_playwright().start()
        self.browser = self.pw.chromium.launch()
        self.page = self.browser.new_page(viewport={'width': 1024, 'height': 1024})

    def png(self, svg_text, px):
        html = ('<html><body style="margin:0;background:transparent">'
                f'<div style="width:{px}px;height:{px}px">{svg_text.replace("<svg ", f"<svg width=\"{px}\" height=\"{px}\" ", 1)}</div>'
                '</body></html>')
        self.page.set_viewport_size({'width': px, 'height': px})
        self.page.set_content(html)
        data = self.page.screenshot(omit_background=True, clip={'x': 0, 'y': 0, 'width': px, 'height': px})
        return Image.open(io.BytesIO(data)).convert('RGBA')

    def close(self):
        self.browser.close()
        self.pw.stop()


def save_png(img, path, opaque=False):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    if opaque:
        bg = Image.new('RGB', img.size, GROUND)
        bg.paste(img, mask=img.split()[3])
        img = bg
    img.save(path, optimize=True)
    print('  wrote', os.path.relpath(path, os.path.dirname(ROOT)))


def save_ico(raster, path, sizes):
    with tempfile.TemporaryDirectory() as td:
        files = []
        for s in sizes:
            f = os.path.join(td, f'{s}.png')
            raster_tile(raster, s).save(f)
            files.append(f)
        subprocess.run(['convert', *files, path], check=True)
    print('  wrote', os.path.relpath(path, os.path.dirname(ROOT)))


def raster_tile(raster, px, shape='rounded', scale=1.0):
    # The thread is under a pixel wide at small sizes and only blurs; drop it.
    return raster.png(tile_svg(shape, scale, thread=px > 40, small=px <= 32), px)


def write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w') as f:
        f.write(text)
    print('  wrote', os.path.relpath(path, os.path.dirname(ROOT)))


def main():
    r = Raster()
    J = lambda *p: os.path.join(ROOT, *p)
    tile_source = (
        '<?xml version="1.0" encoding="UTF-8"?>\n'
        '<!-- Sempa app icon (1.29): the day thread on a terracotta tile. Generated by frontend/scripts/icons.py. -->\n'
        + tile_svg(size=100) + '\n'
    )

    print('SVG sources')
    for p in ('sempa-linux/assets/icon.svg', 'sempa-linux/desktop/icons/hicolor/scalable/apps/ca.sempa.Sempa.svg',
              'frontend/static/favicon.svg', 'frontend/static/icons/favicon.svg'):
        write(J(p), tile_source)
    write(J('sempa-linux/desktop/icons/hicolor/symbolic/apps/ca.sempa.Sempa-symbolic.svg'), symbolic_svg())
    write(J('frontend/static/android-adaptive-foreground.svg'),
          '<?xml version="1.0" encoding="UTF-8"?>\n<!-- Android adaptive foreground (108dp layer). Generated by frontend/scripts/icons.py. -->\n'
          + adaptive_foreground_svg(size=108) + '\n')

    print('Tauri desktop + Windows tiles')
    T = lambda f: J('frontend/src-tauri/icons', f)
    for name, px in [('32x32.png', 32), ('64x64.png', 64), ('128x128.png', 128), ('128x128@2x.png', 256),
                     ('icon.png', 512), ('linux/256x256.png', 256), ('linux/512x512.png', 512),
                     ('Square30x30Logo.png', 30), ('Square44x44Logo.png', 44), ('Square71x71Logo.png', 71),
                     ('Square89x89Logo.png', 89), ('Square107x107Logo.png', 107), ('Square142x142Logo.png', 142),
                     ('Square150x150Logo.png', 150), ('Square284x284Logo.png', 284), ('Square310x310Logo.png', 310),
                     ('StoreLogo.png', 50)]:
        save_png(raster_tile(r, px), T(name))
    save_ico(r, T('icon.ico'), [16, 24, 32, 48, 64, 256])
    big = raster_tile(r, 1024)
    big.save(T('icon.icns'), sizes=[(16, 16), (32, 32), (64, 64), (128, 128), (256, 256), (512, 512), (1024, 1024)])
    print('  wrote', 'sempa/frontend/src-tauri/icons/icon.icns')

    print('Linux')
    for px in (16, 24, 32, 48, 64, 128, 256, 512):
        save_png(raster_tile(r, px), J(f'sempa-linux/desktop/icons/hicolor/{px}x{px}/apps/ca.sempa.Sempa.png'))
    save_png(raster_tile(r, 256), J('sempa-linux/assets/icon-256.png'))
    save_png(raster_tile(r, 512), J('sempa-linux/assets/icon-512.png'))
    save_ico(r, J('sempa-linux/assets/icon.ico'), [16, 32, 48, 256])

    print('Web + PWA')
    for base in ('frontend/static', 'frontend/static/icons'):
        save_ico(r, J(base, 'favicon.ico'), [16, 32, 48, 256])
        save_png(r.png(tile_svg('square'), 180), J(base, 'apple-touch-icon.png'), opaque=True)
    for px in (16, 32, 64):
        save_png(raster_tile(r, px), J('frontend/static', f'favicon-{px}.png'))
    for px in (16, 32, 48):
        save_png(raster_tile(r, px), J('frontend/static/icons', f'favicon-{px}.png'))
    for px in (192, 512):
        save_png(raster_tile(r, px), J('frontend/static', f'android-icon-{px}.png'))
        save_png(raster_tile(r, px), J('frontend/static/icons', f'icon-{px}.png'))
        # Maskable: full-bleed, glyph inside the 80% safe circle.
        save_png(r.png(tile_svg('square', scale=0.86), px), J('frontend/static/icons', f'maskable-{px}.png'), opaque=True)

    print('Android launcher')
    res = J('frontend/android/app/src/main/res')
    for d, k in [('ldpi', 0.75), ('mdpi', 1), ('hdpi', 1.5), ('xhdpi', 2), ('xxhdpi', 3), ('xxxhdpi', 4)]:
        legacy, fg = round(48 * k), round(108 * k)
        save_png(r.png(tile_svg('rounded', thread=legacy > 40), legacy), os.path.join(res, f'mipmap-{d}', 'ic_launcher.png'))
        save_png(r.png(tile_svg('circle', scale=0.9, thread=legacy > 40), legacy), os.path.join(res, f'mipmap-{d}', 'ic_launcher_round.png'))
        save_png(r.png(adaptive_foreground_svg(), fg), os.path.join(res, f'mipmap-{d}', 'ic_launcher_foreground.png'))

    print('iOS')
    save_png(r.png(tile_svg('square'), 1024), J('frontend/resources/ios/AppIcon-1024.png'), opaque=True)

    if os.path.isdir(WEB):
        print('Website (sempa-web)')
        write(os.path.join(WEB, 'assets/favicon.svg'), tile_source)
        save_png(raster_tile(r, 16), os.path.join(WEB, 'assets/favicon-16.png'))
        save_png(raster_tile(r, 32), os.path.join(WEB, 'assets/favicon-32.png'))
        save_png(r.png(tile_svg('square'), 180), os.path.join(WEB, 'assets/apple-touch-icon.png'), opaque=True)
        save_ico(r, os.path.join(WEB, 'favicon.ico'), [16, 32, 48, 256])

    r.close()


if __name__ == '__main__':
    main()
