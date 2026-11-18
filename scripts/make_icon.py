"""Generate the wasaping icon (PNG + macOS .icns) in assets/.

Brand green #81d742 (wasabi.eu) on near-black #171717.
Run: python3 scripts/make_icon.py   (needs Pillow)
"""
import io, os, struct
from PIL import Image, ImageDraw, ImageFilter

OUT = os.path.join(os.path.dirname(__file__), "..", "assets")
S = 1024

def lerp(a, b, t): return tuple(int(a[i] + (b[i] - a[i]) * t) for i in range(3))

def icon():
    img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    # rounded-square background (macOS style grid: 824px body with margins)
    m, r = 100, 185
    bg = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    ImageDraw.Draw(bg).rounded_rectangle((m, m, S - m, S - m), r, fill=(23, 23, 23, 255))
    shadow = bg.filter(ImageFilter.GaussianBlur(18))
    img.alpha_composite(Image.eval(shadow, lambda v: v // 3), (0, 12))
    img.alpha_composite(bg)
    c = S // 2
    # ping rings, composited over the background
    rings = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(rings)
    for rad, a in ((330, 90), (255, 170)):
        d.ellipse((c - rad, c - rad, c + rad, c + rad), outline=(129, 215, 66, a), width=26)
    img.alpha_composite(rings)
    # wasabi ball with radial gradient
    R = 165
    ball = Image.new("RGBA", (2 * R, 2 * R), (0, 0, 0, 0))
    px = ball.load()
    hi, mid, lo = (190, 240, 150), (129, 215, 66), (80, 150, 35)
    for y in range(2 * R):
        for x in range(2 * R):
            dx, dy = x - R, y - R
            if dx * dx + dy * dy > R * R:
                continue
            t = (((x - R * 0.7) ** 2 + (y - R * 0.7) ** 2) ** 0.5) / (R * 1.6)
            col = lerp(hi, mid, min(t / 0.5, 1)) if t < 0.5 else lerp(mid, lo, min((t - 0.5) / 0.5, 1))
            px[x, y] = col + (255,)
    img.alpha_composite(ball, (c - R, c - R))
    return img

def icns(img, path):
    chunks = b""
    for kind, size in ((b"ic07", 128), (b"ic08", 256), (b"ic09", 512), (b"ic10", 1024)):
        buf = io.BytesIO()
        img.resize((size, size), Image.LANCZOS).save(buf, "PNG")
        data = buf.getvalue()
        chunks += kind + struct.pack(">I", len(data) + 8) + data
    with open(path, "wb") as f:
        f.write(b"icns" + struct.pack(">I", len(chunks) + 8) + chunks)

if __name__ == "__main__":
    os.makedirs(OUT, exist_ok=True)
    im = icon()
    im.save(os.path.join(OUT, "icon.png"))
    im.resize((256, 256), Image.LANCZOS).save(os.path.join(OUT, "icon-256.png"))
    icns(im, os.path.join(OUT, "wasaping.icns"))
    print("icone create in", os.path.abspath(OUT))
