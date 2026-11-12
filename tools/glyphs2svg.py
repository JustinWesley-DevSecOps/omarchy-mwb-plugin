#!/usr/bin/env python3
"""Builds design/mwb-icon.svg: the bar icon laid out from the font's own glyph
outlines, using the same numbers as IconMark.qml, so it can be edited as vector
art. Reads the TrueType outlines directly (standard library only).

    python3 tools/glyphs2svg.py [font.ttf] [out.svg]
"""
import struct
import subprocess
import sys

FONT = sys.argv[1] if len(sys.argv) > 1 else subprocess.run(
    ["fc-match", "-f", "%{file}", "JetBrainsMono Nerd Font"], capture_output=True, text=True).stdout
OUT = sys.argv[2] if len(sys.argv) > 2 else "design/mwb-icon.svg"

# ---- layout: keep in step with IconMark.qml (all values in units of s) ----
S = 400.0
MID_Y = 0.5
HEAD_BOX = 0.56
TIP_GAP = 0.015
SHAFT_BOX = 0.5
DIAMOND_BOX = 0.16
DIAMOND_DX = 0.095
DIAMOND_Y = 0.5 - 0.235

SHAFT = 0xF0374   # minus
HEAD_R = 0xF0142  # chevron-right (head of the left arrow)
HEAD_L = 0xF0141  # chevron-left  (head of the right arrow)
DIAMOND = 0xF0A10

data = open(FONT, "rb").read()


def u16(o): return struct.unpack_from(">H", data, o)[0]
def s16(o): return struct.unpack_from(">h", data, o)[0]
def u32(o): return struct.unpack_from(">I", data, o)[0]


tables = {}
for i in range(u16(4)):
    tag = data[12 + i * 16:16 + i * 16].decode()
    tables[tag] = (u32(12 + i * 16 + 8), u32(12 + i * 16 + 12))

head = tables["head"][0]
UPM = u16(head + 18)
LOCA_LONG = s16(head + 50)
NGLYPHS = u16(tables["maxp"][0] + 4)
loca, glyf = tables["loca"][0], tables["glyf"][0]


def glyph_offset(gid):
    if LOCA_LONG:
        a, b = u32(loca + gid * 4), u32(loca + gid * 4 + 4)
    else:
        a, b = u16(loca + gid * 2) * 2, u16(loca + gid * 2 + 2) * 2
    return (glyf + a) if b > a else None


def cmap_lookup(cp):
    base = tables["cmap"][0]
    for i in range(u16(base + 2)):
        pid, eid, off = u16(base + 4 + i * 8), u16(base + 6 + i * 8), u32(base + 8 + i * 8)
        t = base + off
        fmt = u16(t)
        if fmt == 12:
            for g in range(u32(t + 12)):
                start, end, gid = (u32(t + 16 + g * 12), u32(t + 20 + g * 12), u32(t + 24 + g * 12))
                if start <= cp <= end:
                    return gid + cp - start
        elif fmt == 4 and cp < 0x10000:
            segx2 = u16(t + 6)
            ends = t + 14
            starts = ends + segx2 + 2
            deltas = starts + segx2
            ranges = deltas + segx2
            for k in range(segx2 // 2):
                if u16(ends + k * 2) >= cp:
                    if u16(starts + k * 2) > cp:
                        break
                    ro = u16(ranges + k * 2)
                    if ro == 0:
                        return (cp + s16(deltas + k * 2)) & 0xFFFF
                    g = u16(ranges + k * 2 + ro + (cp - u16(starts + k * 2)) * 2)
                    return (g + s16(deltas + k * 2)) & 0xFFFF if g else 0
    raise SystemExit(f"U+{cp:X} not in font")


def contours(gid):
    """Returns a list of contours; each contour is a list of (x, y, on_curve)."""
    off = glyph_offset(gid)
    if off is None:
        return []
    n = s16(off)
    if n < 0:  # composite
        out, p = [], off + 10
        while True:
            flags, sub = u16(p), u16(p + 2)
            p += 4
            if flags & 1:
                dx, dy = s16(p), s16(p + 2); p += 4
            else:
                dx, dy = struct.unpack_from(">bb", data, p); p += 2
            if flags & 8: p += 2
            elif flags & 0x40: p += 4
            elif flags & 0x80: p += 8
            for c in contours(sub):
                out.append([(x + dx, y + dy, on) for x, y, on in c])
            if not flags & 0x20:
                return out
    ends = [u16(off + 10 + i * 2) for i in range(n)]
    npts = ends[-1] + 1 if ends else 0
    p = off + 10 + n * 2
    p += 2 + u16(p)
    flags = []
    while len(flags) < npts:
        f = data[p]; p += 1
        flags.append(f)
        if f & 8:
            r = data[p]; p += 1
            flags.extend([f] * r)
    xs, ys, v = [], [], 0
    for f in flags:
        if f & 2:
            d = data[p]; p += 1; v += d if f & 16 else -d
        elif not f & 16:
            v += s16(p); p += 2
        xs.append(v)
    v = 0
    for f in flags:
        if f & 4:
            d = data[p]; p += 1; v += d if f & 32 else -d
        elif not f & 32:
            v += s16(p); p += 2
        ys.append(v)
    out, start = [], 0
    for e in ends:
        out.append([(xs[i], ys[i], bool(flags[i] & 1)) for i in range(start, e + 1)])
        start = e + 1
    return out


def bbox(cs):
    xs = [x for c in cs for x, _, _ in c]
    ys = [y for c in cs for _, y, _ in c]
    return min(xs), min(ys), max(xs), max(ys)


def path_d(cs, tx):
    """SVG path (quadratic curves) with tx(x, y) -> (X, Y) applied to each point."""
    d = []
    f = lambda pt: "%.2f %.2f" % tx(pt[0], pt[1])
    for c in cs:
        pts = [(x, y) for x, y, _ in c]
        on = [o for _, _, o in c]
        n = len(pts)
        # start on an on-curve point (or a synthetic midpoint)
        if on[0]:
            first, order = pts[0], list(range(1, n)) + [0]
        elif on[-1]:
            first, order = pts[-1], list(range(0, n))
        else:
            first = ((pts[0][0] + pts[-1][0]) / 2, (pts[0][1] + pts[-1][1]) / 2)
            order = list(range(0, n))
        d.append("M" + f(first))
        i, last = 0, first
        seq = [(pts[k], on[k]) for k in order]
        while i < len(seq):
            (p, o) = seq[i]
            if o:
                d.append("L" + f(p)); last = p; i += 1
            else:
                nxt = seq[(i + 1) % len(seq)] if i + 1 < len(seq) else (first, True)
                if i + 1 < len(seq) and not nxt[1]:
                    end = ((p[0] + nxt[0][0]) / 2, (p[1] + nxt[0][1]) / 2)
                    d.append("Q" + f(p) + " " + f(end)); last = end; i += 1
                else:
                    end = nxt[0]
                    d.append("Q" + f(p) + " " + f(end)); last = end; i += 2
        d.append("Z")
    return "".join(d)


def place(cp, box, cx, cy=None, ink_left=None, ink_right=None):
    """Fit the glyph's ink into a square box (like OpticalGlyph's paintedSize),
    centred at (cx, cy), or with its ink's left/right edge pinned."""
    cs = contours(cmap_lookup(cp))
    x0, y0, x1, y1 = bbox(cs)
    k = box / max(x1 - x0, y1 - y0)
    w = (x1 - x0) * k
    if ink_right is not None:
        cx = ink_right - w / 2
    if ink_left is not None:
        cx = ink_left + w / 2
    gx, gy = (x0 + x1) / 2, (y0 + y1) / 2
    return path_d(cs, lambda x, y: (cx + (x - gx) * k, cy - (y - gy) * k))


mid = S * MID_Y
parts = [
    ("shaft-left", place(SHAFT, S * SHAFT_BOX, S * SHAFT_BOX / 2, mid)),
    ("shaft-right", place(SHAFT, S * SHAFT_BOX, S - S * SHAFT_BOX / 2, mid)),
    ("head-left", place(HEAD_R, S * HEAD_BOX, None, mid, ink_right=S * 0.5 - S * TIP_GAP)),
    ("head-right", place(HEAD_L, S * HEAD_BOX, None, mid, ink_left=S * 0.5 + S * TIP_GAP)),
    ("diamond-left", place(DIAMOND, S * DIAMOND_BOX, S * (0.5 - DIAMOND_DX), S * DIAMOND_Y)),
    ("diamond-right", place(DIAMOND, S * DIAMOND_BOX, S * (0.5 + DIAMOND_DX), S * DIAMOND_Y)),
]

svg = [f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {S:g} {S:g}" width="{S:g}" height="{S:g}">
  <title>Mouse Without Borders bar icon</title>
  <desc>Outlines taken from the JetBrainsMono Nerd Font glyphs U+F0374 (shaft), U+F0142 and U+F0141 (heads) and U+F0A10 (diamond), laid out as in IconMark.qml. Each piece is its own path so it can be moved, resized or replaced.</desc>
  <!-- guides: delete or hide -->
  <g id="guides" fill="none" stroke="#7aa" stroke-width="1" stroke-dasharray="4 4" opacity="0.6">
    <rect x="0.5" y="0.5" width="{S - 1:g}" height="{S - 1:g}"/>
    <line x1="0" y1="{mid:g}" x2="{S:g}" y2="{mid:g}"/>
    <line x1="{S / 2:g}" y1="0" x2="{S / 2:g}" y2="{S:g}"/>
  </g>
  <g id="icon" fill="#f5d9a8">''']
for name, d in parts:
    svg.append(f'    <path id="{name}" d="{d}"/>')
svg.append("  </g>\n</svg>\n")
open(OUT, "w").write("\n".join(svg))
print("wrote", OUT, "UPM", UPM)
